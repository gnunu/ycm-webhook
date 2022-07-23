package configuration

import (
	"context"
	"os"

	"github.com/openyurtio/pkg/webhooks/pod-validator/certs"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	WebhookNamespace   = getEnv("WEBHOOK_NAMESPACE", "kube-system")
	ValidateConfigName = getEnv("WEBHOOK_CONFIGURATION", "ycm-webhook-configuration")
	WebhookService     = getEnv("WEBHOOK_SERVICE", "ycm-webhook")
	WebhookName        = getEnv("VALIDATE_WEBHOOK_NAME", "ycm-validating.openyurt.io")
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func generateValidateConfig(path *string, certset *certs.Certs) *admissionregistrationv1.ValidatingWebhookConfiguration {
	fail := admissionregistrationv1.Fail
	sideEffects := admissionregistrationv1.SideEffectClassNone
	validateconfig := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: ValidateConfigName,
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{{
			Name: WebhookName,
			ClientConfig: admissionregistrationv1.WebhookClientConfig{
				CABundle: certset.CACert,
				Service: &admissionregistrationv1.ServiceReference{
					Name:      WebhookService,
					Namespace: WebhookNamespace,
					Path:      path,
				},
			},
			Rules: []admissionregistrationv1.RuleWithOperations{
				{Operations: []admissionregistrationv1.OperationType{
					admissionregistrationv1.Create,
					admissionregistrationv1.Delete,
					admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
					},
				}},
			FailurePolicy:           &fail,
			SideEffects:             &sideEffects,
			AdmissionReviewVersions: []string{"v1"},
		}},
	}
	return validateconfig
}

func EnsureValidateConfiguration(clientset *kubernetes.Clientset, path *string, certset *certs.Certs) {
	validateconfig := generateValidateConfig(path, certset)
	if _, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
		context.TODO(), ValidateConfigName, v1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("validatewebhookconfiguratiion %s not found, create it.", ValidateConfigName)
			if _, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(
				context.TODO(), validateconfig, v1.CreateOptions{}); err != nil {
				klog.Fatal(err)
			}
		}
	} else {
		klog.Infof("validatewebhookconfiguratiion %s already exists, update it.", ValidateConfigName)
		if _, err = clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(
			context.TODO(), validateconfig, v1.UpdateOptions{}); err != nil {
			klog.Fatal(err)
		}
	}
}
