package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type Context struct {
	clientset kubernetes.Interface
}

type validation struct {
	Valid  bool
	Reason string
}

type PodValidator struct {
	request *admissionv1.AdmissionRequest
	pod     *corev1.Pod
	node    *corev1.Node
}

func (pv PodValidator) getNode() (*corev1.Node, error) {
	return nil, nil
}

func (pv PodValidator) ValidateReview() (*admissionv1.AdmissionReview, error) {
	pod, err := pv.Pod()
	if err != nil {
		e := fmt.Sprintf("could not parse pod in admission review request: %v", err)
		return reviewResponse(pv.request.UID, false, http.StatusBadRequest, e), err
	}

	pv.pod = pod

	val, err := pv.Validate()

	if err != nil {
		e := fmt.Sprintf("could not validate pod: %v", err)
		return reviewResponse(pv.request.UID, false, http.StatusBadRequest, e), err
	}

	if !val.Valid {
		return reviewResponse(pv.request.UID, false, http.StatusForbidden, val.Reason), nil
	}

	return reviewResponse(pv.request.UID, true, http.StatusAccepted, val.Reason), nil
}

// extracts pod from admission request
func (pv PodValidator) Pod() (*corev1.Pod, error) {
	if pv.request.Kind.Kind != "Pod" {
		return nil, fmt.Errorf("only pods are supported here")
	}

	p := corev1.Pod{}
	if err := json.Unmarshal(pv.request.Object.Raw, &p); err != nil {
		return nil, err
	}

	return &p, nil
}

// ValidatePod returns true if a pod is valid
func (pv PodValidator) Validate() (validation, error) {
	var podName string
	if pv.pod.Name != "" {
		podName = pv.pod.Name
	} else {
		if pv.pod.ObjectMeta.GenerateName != "" {
			podName = pv.pod.ObjectMeta.GenerateName
		}
	}
	klog.Info("pod_name", podName)

	return validation{Valid: true, Reason: "valid pod"}, nil
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

	pv := PodValidator{
		request: in.Request,
	}

	klog.Info(fmt.Sprintf("%v", in.Request))

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

func Register() {
	http.HandleFunc("/ycm-validate-pods", ServeValidatePods)
	http.HandleFunc("/ycm-webhook-health", ServeHealth)

	if os.Getenv("TLS") == "true" {
		cert := "/etc/ycm-webhook/tls/tls.crt"
		key := "/etc/ycm-webhook/tls/tls.key"
		klog.Info("Listening on port 443...")
		klog.Fatal(http.ListenAndServeTLS(":443", cert, key, nil))
	} else {
		klog.Info("Listening on port 8080...")
		klog.Fatal(http.ListenAndServe(":8080", nil))
	}
}
