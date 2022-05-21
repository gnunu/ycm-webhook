package configuration

import (
	"context"
	"os"

	"github.com/openyurtio/pkg/webhooks/pod-validator/certs"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var (
	WebhookNamespace, _   = os.LookupEnv("WEBHOOK_NAMESPACE")
	ValidateConfigName, _ = os.LookupEnv("WEBHOOK_CONFIG")
	WebhookService, _     = os.LookupEnv("WEBHOOK_SERVICE")
	WebhookName, _        = os.LookupEnv("VALIDATE_WEBHOOK_NAME")
)

func CreateValidateConfiguration(clientset *kubernetes.Clientset, path *string, certset *certs.Certs) {
	fail := admissionregistrationv1.Fail
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
			FailurePolicy: &fail,
		}},
	}

	if _, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(context.TODO(), validateconfig, v1.CreateOptions{}); err != nil {
		klog.Fatal(err)
	}
}
