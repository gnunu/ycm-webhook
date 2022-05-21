package secret

import (
	"context"

	"github.com/openyurtio/pkg/webhooks/pod-validator/certs"
	"github.com/openyurtio/pkg/webhooks/pod-validator/configuration"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func getSecretName() string {
	return "ycm-webhook-certs"
}

func getNameSpace() string {
	return configuration.WebhookNamespace
}

func CreateSecret(clientset *kubernetes.Clientset, certset *certs.Certs) {
	secret, err := clientset.CoreV1().Secrets(getNameSpace()).Get(context.TODO(), getSecretName(), v1.GetOptions{})
	if err != nil {
		klog.Fatal(err)
	}
	secret.Data = map[string][]byte{
		certs.CAKeyName:       certset.CAKey,
		certs.CACertName:      certset.CACert,
		certs.ServerKeyName:   certset.Key,
		certs.ServerKeyName2:  certset.Key,
		certs.ServerCertName:  certset.Cert,
		certs.ServerCertName2: certset.Cert,
	}
}
