package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/openyurtio/pkg/controller/poolcoordinator/client"
	"github.com/openyurtio/pkg/controller/poolcoordinator/constant"
	"github.com/openyurtio/pkg/controller/poolcoordinator/lister"
	"github.com/openyurtio/pkg/controller/poolcoordinator/utils"
	"github.com/wI2L/jsondiff"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	leaselisterv1 "k8s.io/client-go/listers/coordination/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

const (
	msgNodeAutonomy                      string = "node autonomy annotated, eviction aborted"
	msgPodAvailableNode                  string = "pod should exist on the specific node, eviction aborted"
	msgPodAvailablePoolAndNodeIsAlive    string = "node is actually alive in a pool, eviction aborted"
	msgPodAvailablePoolAndNodeIsNotAlive string = "node is not alive in a pool, eviction approved"
	msgPodDeleteValidated                string = "pod deletion validated"
)

var (
	ValidatePath string = "/pool-coordinator-webhook-validate"
	MutatePath   string = "/pool-coordinator-webhook-mutate"
	HealthPath   string = "/pool-coordinator-webhook-health"

	nodeLister  listerv1.NodeLister
	leaseLister leaselisterv1.LeaseNamespaceLister
)

type validation struct {
	Valid  bool
	Reason string
}

type PodAdmission struct {
	request *admissionv1.AdmissionRequest
	pod     *corev1.Pod
	node    *corev1.Node
}

// extracts pod from admission request
func (pv *PodAdmission) getPod() error {
	if err := json.Unmarshal(pv.request.OldObject.Raw, pv.pod); err != nil {
		klog.Error(err)
		return err
	}

	return nil
}

func (pv *PodAdmission) userIsNodeController() bool {
	return strings.Contains(pv.request.UserInfo.Username, "system:serviceaccount:kube-system:node-controller")
}

func (pv *PodAdmission) NodeIsInAutonomy(node *corev1.Node) bool {
	if node.Annotations != nil && node.Annotations[constant.AnnotationKeyNodeAutonomy] == "true" {
		return true
	}
	return false
}

func (pv *PodAdmission) NodeIsAlive(node *corev1.Node) bool {
	lease, err := leaseLister.Get(node.Name)
	if err != nil {
		klog.Error(err)
		return false
	}
	diff := time.Now().Sub(lease.GetCreationTimestamp().Time)
	if diff.Seconds() > 40 {
		return false
	}
	return true
}

func (pv *PodAdmission) getNode() error {
	nodeName := pv.pod.Spec.NodeName
	klog.Infof("nodeName: %s", nodeName)
	node, err := nodeLister.Get(nodeName)
	pv.node = node
	return err
}

func (pv *PodAdmission) ValidateReview() (*admissionv1.AdmissionReview, error) {
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

// ValidateDel returns true if a pod is valid to delete/evict
func (pv *PodAdmission) validateDel() (validation, error) {
	if pv.request.Operation == admissionv1.Delete {
		if pv.userIsNodeController() {
			// node is autonomy annotated
			if pv.NodeIsInAutonomy(pv.node) {
				return validation{Valid: false, Reason: msgNodeAutonomy}, nil
			}

			if pv.pod.Annotations != nil {
				// pod has annotation of node available
				if pv.pod.Annotations[constant.PodAvailableAnnotation] == "node" {
					return validation{Valid: false, Reason: msgPodAvailableNode}, nil
				}

				if pv.pod.Annotations[constant.PodAvailableAnnotation] == "pool" {
					if pv.NodeIsAlive(pv.node) {
						return validation{Valid: false, Reason: msgPodAvailablePoolAndNodeIsAlive}, nil
					} else {
						return validation{Valid: true, Reason: msgPodAvailablePoolAndNodeIsNotAlive}, nil
					}
				}
			}
		}
	}
	return validation{Valid: true, Reason: msgPodDeleteValidated}, nil
}

func (pv *PodAdmission) MutateAddToleration() ([]byte, error) {
	toadd := []corev1.Toleration{
		{Key: "node.kubernetes.io/unreachable",
			Operator: "Exists",
			Effect:   "NoExecute"},
		{Key: "node.kubernetes.io/not-ready",
			Operator: "Exists",
			Effect:   "NoExecute"},
	}
	tols := pv.pod.Spec.Tolerations
	merged, changed := utils.MergeTolerations(tols, toadd)
	if !changed {
		return nil, nil
	}

	mpod := pv.pod.DeepCopy()
	mpod.Spec.Tolerations = merged

	// generate json patch
	patch, err := jsondiff.Compare(pv.pod, mpod)
	if err != nil {
		return nil, err
	}

	patchb, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	return patchb, nil
}

func (pv *PodAdmission) MutateReview() (*admissionv1.AdmissionReview, error) {
	if pv.request.Kind.Kind != "Pod" {
		err := fmt.Errorf("only pods are supported here")
		return reviewResponse(pv.request.UID, false, http.StatusBadRequest, ""), err
	}

	if pv.request.Operation != admissionv1.Create && pv.request.Operation != admissionv1.Update {
		reason := fmt.Sprintf("Operation %v is accepted always", pv.request.Operation)
		return reviewResponse(pv.request.UID, true, http.StatusAccepted, reason), nil
	}

	err := pv.getPod()
	if err != nil {
		e := fmt.Sprintf("could not parse pod in admission review request: %v", err)
		return reviewResponse(pv.request.UID, false, http.StatusBadRequest, e), err
	}

	if pv.pod.Annotations == nil || pv.pod.Annotations[constant.PodAvailableAnnotation] != "true" {
		return reviewResponse(pv.request.UID, true, http.StatusAccepted, "no need of mutation"), nil
	}

	// add tolerations if not yet
	val, err := pv.MutateAddToleration()
	if err != nil {
		return reviewResponse(pv.request.UID, true, http.StatusAccepted, "could not merge tolerations"), err
	}
	if val == nil {
		return reviewResponse(pv.request.UID, true, http.StatusAccepted, "tolerations already existed"), nil
	}

	return patchReviewResponse(pv.request.UID, val)
}

func reviewResponse(uid types.UID, allowed bool, httpCode int32, reason string) *admissionv1.AdmissionReview {
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

// patchReviewResponse builds an admission review with given json patch
func patchReviewResponse(uid types.UID, patch []byte) (*admissionv1.AdmissionReview, error) {
	patchType := admissionv1.PatchTypeJSONPatch

	return &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AdmissionReview",
			APIVersion: "admission.k8s.io/v1",
		},
		Response: &admissionv1.AdmissionResponse{
			UID:       uid,
			Allowed:   true,
			PatchType: &patchType,
			Patch:     patch,
		},
	}, nil
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

	pv := &PodAdmission{
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

// ServeMutatePods mutates an admission request and then writes an admission
func ServeMutatePods(w http.ResponseWriter, r *http.Request) {
	klog.Info("uri", r.RequestURI)
	klog.Info("received validation request")

	in, err := parseRequest(*r)
	if err != nil {
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pv := &PodAdmission{
		request: in.Request,
		pod:     &corev1.Pod{},
	}

	klog.Infof("name: %s, namespace: %s, operation: %s, from: %v",
		in.Request.Name, in.Request.Namespace, in.Request.Operation, &in.Request.UserInfo)

	out, err := pv.MutateReview()

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

const (
	CertDir string = "/tmp/k8s-webhook-server/serving-certs"
)

func Run(nLister listerv1.NodeLister, lLister leaselisterv1.LeaseNamespaceLister) {
	nodeLister = nLister
	leaseLister = lLister

	http.HandleFunc(ValidatePath, ServeValidatePods)
	http.HandleFunc(MutatePath, ServeMutatePods)
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

	client := client.GetClientFromCluster()
	stopper := make(chan (struct{}))
	nodeLister = lister.CreateNodeLister(client, stopper, nil, nil, nil)

	klog.Info("Listening on port 443...")
	klog.Fatal(http.ListenAndServeTLS(":443", cert, key, nil))
}
