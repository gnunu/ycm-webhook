package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/openyurtio/pkg/webhooks/pod-validator/client"
	"github.com/openyurtio/pkg/webhooks/pod-validator/nodes"
	"github.com/openyurtio/pkg/webhooks/pod-validator/utils"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	podaffinityannotation string = "podaffinityannotation"
	podaffinitynode       string = "podnodeaffinity"
	podaffinitypool       string = "podpoolaffinity"
)

var (
	ValidatePath string = "/pod-coordinator-webhook-validate"
	HealthPath   string = "/pod-coordinator-webhook-health"
	clientset    *kubernetes.Clientset
)

type validation struct {
	Valid  bool
	Reason string
}

type PodValidator struct {
	request *admissionv1.AdmissionRequest
	pod     *corev1.Pod
	node    *corev1.Node
}

// extracts pod from admission request
func (pv *PodValidator) getPod() error {
	if err := json.Unmarshal(pv.request.OldObject.Raw, pv.pod); err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

func (pv *PodValidator) userIsNodeController() bool {
	return strings.Contains(pv.request.UserInfo.Username, "system:serviceaccount:kube-system:node-controller")
}

func (pv *PodValidator) getNode() error {
	nodeName := pv.pod.Spec.NodeName
	klog.Infof("nodeName: %s", nodeName)
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, v1.GetOptions{})
	pv.node = node
	return err
}

func (pv *PodValidator) ValidateReview() (*admissionv1.AdmissionReview, error) {
	if pv.request.Kind.Kind != "Pod" {
		err := fmt.Errorf("only pods are supported here")
		return reviewResponse(pv.request.UID, false, http.StatusBadRequest, ""), err
	}

	if pv.request.Operation != admissionv1.Delete {
		reason := fmt.Sprintf("Operation %v is accepted always", pv.request.Operation)
		return reviewResponse(pv.request.UID, true, http.StatusAccepted, reason), nil
	}

	err := pv.getPod()
	if err != nil {
		e := fmt.Sprintf("could not parse pod in admission review request: %v", err)
		return reviewResponse(pv.request.UID, false, http.StatusBadRequest, e), err
	}

	err = pv.getNode()
	if err != nil {
		e := fmt.Sprintf("could not get node object: %s", pv.pod.Spec.NodeName)
		return reviewResponse(pv.request.UID, false, http.StatusBadRequest, e), err
	}

	val, err := pv.validateDel()

	if err != nil {
		e := fmt.Sprintf("could not validate pod: %v", err)
		return reviewResponse(pv.request.UID, false, http.StatusBadRequest, e), err
	}

	if !val.Valid {
		return reviewResponse(pv.request.UID, false, http.StatusForbidden, val.Reason), nil
	}

	return reviewResponse(pv.request.UID, true, http.StatusAccepted, val.Reason), nil
}

// ValidatePod returns true if a pod is valid
func (pv *PodValidator) validateDel() (validation, error) {
	if pv.request.Operation == admissionv1.Delete {
		if pv.userIsNodeController() {
			if nodes.NodeIsInAutonomy(pv.node) {
				return validation{Valid: false, Reason: "node autonomy labeled"}, nil
			}
			// case 1: pod has annotation of nodeaffinity
			// return validation{Valid: false, Reason: "pod has annotation of nodeautonomy"}, nil

			// case 2: pod has annotation of poolaffinity
			// if the node has lease object proxyed
			// return validation{Valid: false, Reason: "pod has annotatiuon of poolautonomy, and the node is alive"}, nil
			// else if the node is really down
			// return validation{Valid: true, Reason: "pod has annotatiuon of poolautonomy, and the node is down, allow transition"}, nil
			// when pod is poolautonomy annotated, the pod should be updated with node affinity
		}
	}
	return validation{Valid: true, Reason: "validated pod deletion"}, nil
}

func reviewResponse(uid types.UID, allowed bool, httpCode int32,
	reason string) *admissionv1.AdmissionReview {
	return &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:     uid,
			Allowed: allowed,
			Result: &metav1.Status{
				Code:    httpCode,
				Message: reason,
			},
		},
	}
}

// ServeHealth returns 200 when things are good
func ServeHealth(w http.ResponseWriter, r *http.Request) {
	klog.Info("uri", r.RequestURI)
	fmt.Fprint(w, "OK")
}

// ServeValidatePods validates an admission request and then writes an admission
func ServeValidatePods(w http.ResponseWriter, r *http.Request) {
	klog.Info("uri", r.RequestURI)
	klog.Info("received validation request")

	in, err := parseRequest(*r)
	if err != nil {
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pv := &PodValidator{
		request: in.Request,
		pod:     &corev1.Pod{},
	}

	klog.Infof("name: %s, namespace: %s, operation: %s, from: %v",
		in.Request.Name, in.Request.Namespace, in.Request.Operation, &in.Request.UserInfo)

	out, err := pv.ValidateReview()

	if err != nil {
		e := fmt.Sprintf("could not generate admission response: %v", err)
		klog.Error(e)
		http.Error(w, e, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	jout, err := json.Marshal(out)
	if err != nil {
		e := fmt.Sprintf("could not parse admission response: %v", err)
		klog.Error(e)
		http.Error(w, e, http.StatusInternalServerError)
		return
	}

	klog.Info("sending response")
	klog.Infof("%s", jout)
	fmt.Fprintf(w, "%s", jout)
}

// parseRequest extracts an AdmissionReview from an http.Request if possible
func parseRequest(r http.Request) (*admissionv1.AdmissionReview, error) {
	if r.Header.Get("Content-Type") != "application/json" {
		return nil, fmt.Errorf("Content-Type: %q should be %q",
			r.Header.Get("Content-Type"), "application/json")
	}

	bodybuf := new(bytes.Buffer)
	bodybuf.ReadFrom(r.Body)
	body := bodybuf.Bytes()

	if len(body) == 0 {
		return nil, fmt.Errorf("admission request body is empty")
	}

	var a admissionv1.AdmissionReview

	if err := json.Unmarshal(body, &a); err != nil {
		return nil, fmt.Errorf("could not parse admission review request: %v", err)
	}

	if a.Request == nil {
		return nil, fmt.Errorf("admission review can't be used: Request field is nil")
	}

	return &a, nil
}

func rotateCertIfNecessary() error {
	return nil
}

const (
	CertDir string = "/tmp/k8s-webhook-server/serving-certs"
)

func RegisterWebhook() {
	http.HandleFunc(ValidatePath, ServeValidatePods)
	http.HandleFunc(HealthPath, ServeHealth)

	err := utils.EnsureDir(CertDir)
	if err != nil {
		klog.Error(err)
	}
	cert := CertDir + "/tls.crt"
	key := CertDir + "/tls.key"

	for {
		if utils.FileExists(cert) && utils.FileExists(key) {
			klog.Info("tls key and cert ok.")
			break
		} else {
			klog.Info("Wating for tls key and cert...")
			time.Sleep(time.Second)
		}
	}

	// rotate cert if necessary
	err = rotateCertIfNecessary()
	if err != nil {
		klog.Fatal(err)
	}

	klog.Info("Listening on port 443...")
	klog.Fatal(http.ListenAndServeTLS(":443", cert, key, nil))
}

func doNothing(obj interface{}) {
	klog.Info("do nothing")
}

func CreateInformers() {
	//clientset = client.GetClientFromEnv(os.Getenv("HOME") + "/.kube/config")
	clientset = client.GetClientFromCluster()

	// factory := informers.NewSharedInformerFactoryWithOptions(clientset, 10*time.Second, options)
	factory := informers.NewSharedInformerFactory(clientset, 10*time.Second)
	klog.Infof("factory: %v\n", factory)
	podInformer := factory.Core().V1().Pods()
	podLister := podInformer.Lister()
	pInformer := podInformer.Informer()
	pInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nil,
		UpdateFunc: nil,
		DeleteFunc: nil,
	})
	klog.Infof("podInformer: %v\n", podInformer)
	stopCh := make(chan struct{})
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)
	pods, err := podLister.List(labels.Everything())
	if err != nil {
		klog.Error(err)
	} else {
		klog.Infof("%v", len(pods))
	}
	/*
		nodesInformer := factory.Core().V1().Nodes().Informer()
		klog.Infof("nodesInformer: %v\n", nodesInformer)
		leasesInformer := factory.Coordination().V1().Leases().Informer()
		fmt.Print(podsInformer)
		fmt.Print(nodesInformer)
		fmt.Print(leasesInformer)
	*/
}
