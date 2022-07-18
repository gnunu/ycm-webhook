package secret

import (
	"context"
	"flag"
	"fmt"
	"testing"

	"github.com/openyurtio/pkg/webhooks/pod-validator/certs"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func TestSecret(t *testing.T) {
	ns := "kube-system"
	sn := "ycm-webhook-certs"
	kubeconfig := flag.String("kubeconfig", "/home/nunu/.kube/config", "absolute path to the kubeconfig file")
	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		t.Fatalf(err.Error())
	}

	certs := certs.GenerateCerts(ns, ns)

	CreateSecret(clientset, certs)

	secret, err := clientset.CoreV1().Secrets(ns).Get(context.TODO(), sn, v1.GetOptions{})

	if err != nil {
		t.Fatalf(err.Error())
	}

	if secret == nil {
		t.Errorf("got nil, wanted not nil")
	} else {
		fmt.Println(secret)
	}
}
