package lister

import (
	"time"

	"github.com/openyurtio/pkg/webhooks/pod-validator/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	leaselisterv1 "k8s.io/client-go/listers/coordination/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var (
	//podLister   listerv1.PodLister
	nodeLister  listerv1.NodeLister
	leaseLister leaselisterv1.LeaseNamespaceLister
)

func NodeLister() listerv1.NodeLister {
	return nodeLister
}

func LeaseLister() leaselisterv1.LeaseNamespaceLister {
	return leaseLister
}

func CreateListers() {
	//clientset = client.GetClientFromEnv(os.Getenv("HOME") + "/.kube/config")
	clientset := client.GetClientFromCluster()

	stopCh := make(chan struct{})
	factory := informers.NewSharedInformerFactory(clientset, 10*time.Second)
	klog.Infof("factory: %v\n", factory)
	/*
		podInformer := factory.Core().V1().Pods()
		podLister = podInformer.Lister()
		pInformer := podInformer.Informer()
		pInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    nil,
			UpdateFunc: nil,
			DeleteFunc: nil,
		})
	*/
	nodeInformer := factory.Core().V1().Nodes()
	nodeLister = nodeInformer.Lister()
	nInformer := nodeInformer.Informer()
	nInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nil,
		UpdateFunc: nil,
		DeleteFunc: nil,
	})
	leaseInformer := factory.Coordination().V1().Leases()
	leaseLister = leaseInformer.Lister().Leases(corev1.NamespaceNodeLease)
	lInformer := leaseInformer.Informer()
	lInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nil,
		UpdateFunc: nil,
		DeleteFunc: nil,
	})

	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)
}
