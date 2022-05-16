package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	v1beta1 "k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
)

type Admiss struct {
	Request *admissionv1.AdmissionRequest
}

func (a Admiss) ValidatePodReview() (*admissionv1.AdmissionReview, error) {
	pod, err := a.Pod()
	if err != nil {
		e := fmt.Sprintf("could not parse pod in admission review request: %v", err)
		return reviewResponse(a.Request.UID, false, http.StatusBadRequest, e), err
	}

	v := NewValidator()
	val, err := v.ValidatePod(pod)
	if err != nil {
		e := fmt.Sprintf("could not validate pod: %v", err)
		return reviewResponse(a.Request.UID, false, http.StatusBadRequest, e), err
	}

	if !val.Valid {
		return reviewResponse(a.Request.UID, false, http.StatusForbidden, val.Reason), nil
	}

	return reviewResponse(a.Request.UID, false, http.StatusAccepted, "always invalid pod"), nil
}

func (a Admiss) ValidateEvictionReview() (*admissionv1.AdmissionReview, error) {
	evic, err := a.Eviction()
	if err != nil {
		e := fmt.Sprintf("could not parse pod in admission review request: %v", err)
		return reviewResponse(a.Request.UID, false, http.StatusBadRequest, e), err
	}

	v := NewValidator()
	val, err := v.ValidateEviction(evic)
	if err != nil {
		e := fmt.Sprintf("could not validate eviction: %v", err)
		return reviewResponse(a.Request.UID, false, http.StatusBadRequest, e), err
	}

	if !val.Valid {
		return reviewResponse(a.Request.UID, false, http.StatusForbidden, val.Reason), nil
	}

	return reviewResponse(a.Request.UID, true, http.StatusAccepted, "valid eviction"), nil
}

// extracts pod from admission request
func (a Admiss) Pod() (*corev1.Pod, error) {
	if a.Request.Kind.Kind != "Pod" {
		return nil, fmt.Errorf("only pods are supported here")
	}

	p := corev1.Pod{}
	if err := json.Unmarshal(a.Request.Object.Raw, &p); err != nil {
		return nil, err
	}

	return &p, nil
}

// extracts eviction from admission request
func (a Admiss) Eviction() (*v1beta1.Eviction, error) {
	if a.Request.Kind.Kind != "Eviction" {
		return nil, fmt.Errorf("only evictions are supported here")
	}

	p := v1beta1.Eviction{}
	if err := json.Unmarshal(a.Request.Object.Raw, &p); err != nil {
		return nil, err
	}

	return &p, nil

}

type Validator struct {
}

// NewValidator returns an initialised instance of Validator
func NewValidator() *Validator {
	return &Validator{}
}

// podValidators is an interface used to group functions mutating pods
type podValidator interface {
	Validate(*corev1.Pod) (validation, error)
	Name() string
}

// podValidators is an interface used to group functions mutating pods
type evictionValidator interface {
	Validate(*v1beta1.Eviction) (validation, error)
	Name() string
}

type validation struct {
	Valid  bool
	Reason string
}

// ValidatePod returns true if a pod is valid
func (v *Validator) ValidatePod(pod *corev1.Pod) (validation, error) {
	var podName string
	if pod.Name != "" {
		podName = pod.Name
	} else {
		if pod.ObjectMeta.GenerateName != "" {
			podName = pod.ObjectMeta.GenerateName
		}
	}
	klog.Info("pod_name", podName)

	// list of all validations to be applied to the pod
	validations := []podValidator{}

	// apply all validations
	for _, v := range validations {
		var err error
		vp, err := v.Validate(pod)
		if err != nil {
			return validation{Valid: false, Reason: err.Error()}, err
		}
		if !vp.Valid {
			return validation{Valid: false, Reason: vp.Reason}, err
		}
	}

	return validation{Valid: true, Reason: "valid pod"}, nil
}

func (v *Validator) ValidateEviction(evic *v1beta1.Eviction) (validation, error) {
	var podName string
	if evic.Name != "" {
		podName = evic.Name
	} else {
		return validation{Valid: false, Reason: "no name"}, nil
	}
	klog.Info("eviction name", podName)

	// get pod's node, if node is autonomy, then reject

	return validation{Valid: true, Reason: "valid eviction"}, nil
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

func Register() {
	http.HandleFunc("/ycm-validate-evictions", ServeValidateEvictions)
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

// ServeHealth returns 200 when things are good
func ServeHealth(w http.ResponseWriter, r *http.Request) {
	klog.Info("uri", r.RequestURI)
	fmt.Fprint(w, "OK")
}

// ServeValidateEvictions validates an admission request and then writes an admission
func ServeValidateEvictions(w http.ResponseWriter, r *http.Request) {
	klog.Info("uri", r.RequestURI)
	klog.Info("received validation request")

	in, err := parseRequest(*r)
	if err != nil {
		klog.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	adm := Admiss{
		Request: in.Request,
	}

	out, err := adm.ValidateEvictionReview()

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

	adm := Admiss{
		Request: in.Request,
	}

	out, err := adm.ValidatePodReview()

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
