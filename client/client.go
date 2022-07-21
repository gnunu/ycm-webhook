package client

import (
	"flag"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func GetClientFromEnv(cf string) *kubernetes.Clientset {
	kubeconfig := flag.String("kubeconfig", cf, "absolute path to the kubeconfig file")
	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)

	if err != nil {
		klog.Fatal(err)
	}

	return clientset
}

func GetClientFromCluster() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatal(err)
	}

	return clientset
}
