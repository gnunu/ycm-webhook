package lister

import (
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	leaselisterv1 "k8s.io/client-go/listers/coordination/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	resyncInt = 5 * time.Second
)

var (
	factory informers.SharedInformerFactory
)

type ACallback func(interface{})
type UCallback func(interface{}, interface{})

func CreateNodeLister(client *kubernetes.Clientset, stopper chan (struct{}), afunc ACallback, ufunc UCallback, dfunc ACallback) listerv1.NodeLister {
	if reflect.ValueOf(factory).IsNil() {
		factory = informers.NewSharedInformerFactory(client, resyncInt)
	}
	nodeInformer := factory.Core().V1().Nodes()
	nodeLister := nodeInformer.Lister()
	nInformer := nodeInformer.Informer()
	nInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    afunc,
		UpdateFunc: ufunc,
		DeleteFunc: dfunc,
	})
	nInformer.Run(stopper)
	return nodeLister
}

func CreatePodLister(client *kubernetes.Clientset, stopper chan (struct{}), afunc ACallback, ufunc UCallback, dfunc ACallback) listerv1.PodLister {
	if reflect.ValueOf(factory).IsNil() {
		factory = informers.NewSharedInformerFactory(client, resyncInt)
	}
	podInformer := factory.Core().V1().Pods()
	podLister := podInformer.Lister()
	pInformer := podInformer.Informer()
	pInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nil,
		UpdateFunc: nil,
		DeleteFunc: nil,
	})
	pInformer.Run(stopper)
	return podLister
}

func CreateLeaseLister(client *kubernetes.Clientset, stopper chan (struct{}), acb ACallback, ucb UCallback, dcb ACallback) leaselisterv1.LeaseNamespaceLister {
	if reflect.ValueOf(factory).IsNil() {
		factory = informers.NewSharedInformerFactory(client, resyncInt)
	}
	leaseInformer := factory.Coordination().V1().Leases()
	leaseLister := leaseInformer.Lister().Leases(corev1.NamespaceNodeLease)
	lInformer := leaseInformer.Informer()
	lInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    acb,
		UpdateFunc: ucb,
		DeleteFunc: dcb,
	})
	lInformer.Run(stopper)
	return leaseLister
}
