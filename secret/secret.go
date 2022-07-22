package secret

import (
	"context"

	"github.com/openyurtio/pkg/webhooks/pod-validator/certs"
	"github.com/openyurtio/pkg/webhooks/pod-validator/configuration"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ns := getNameSpace()
	sn := getSecretName()
	found := false
	secret, err := clientset.CoreV1().Secrets(ns).Get(context.TODO(), sn, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			secret = &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ns,
					Name:      sn,
				},
			}
		} else {
			klog.Error(err)
			return
		}
	} else {
		found = true
		klog.Infof("secret %s already existed.", secret.Name)
	}
	secret.Data = map[string][]byte{
		certs.CAKeyName:      certset.CAKey,
		certs.CACertName:     certset.CACert,
		certs.ServerKeyName:  certset.Key,
		certs.ServerCertName: certset.Cert,
	}
	if found {
		_, err = clientset.CoreV1().Secrets(ns).Update(context.TODO(), secret, v1.UpdateOptions{})
	} else {
		_, err = clientset.CoreV1().Secrets(ns).Create(context.TODO(), secret, v1.CreateOptions{})
	}
	if err != nil {
		klog.Error(err)
	}
}
