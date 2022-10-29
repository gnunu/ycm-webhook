package controller

import (
	"context"
	"sync"

	"github.com/openyurtio/pkg/controller/poolcoordinator/client"
	"github.com/openyurtio/pkg/controller/poolcoordinator/constant"
	"github.com/openyurtio/pkg/controller/poolcoordinator/lister"
	"github.com/openyurtio/pkg/controller/poolcoordinator/utils"
	"github.com/openyurtio/pkg/controller/poolcoordinator/webhook"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	leaselisterv1 "k8s.io/client-go/listers/coordination/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

type Controller struct {
	client      *kubernetes.Clientset
	nodeLister  listerv1.NodeLister
	leaseLister leaselisterv1.LeaseNamespaceLister
	nodepoolMap *utils.NodepoolMap
}

type LeaseDelegatedCounter struct {
	v    map[string]int
	lock sync.RWMutex
}

var (
	ctl *Controller

	ldc *LeaseDelegatedCounter
)

func (dc *LeaseDelegatedCounter) Inc(name string) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	if dc.v[name] >= constant.LeaseDelegationThreshold {
		return
	}
	dc.v[name] += 1
}

func (dc *LeaseDelegatedCounter) Dec(name string) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	if dc.v[name] > 0 {
		dc.v[name] -= 1
	}
}

func (dc *LeaseDelegatedCounter) Reset(name string) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	dc.v[name] = 0
}

func (dc *LeaseDelegatedCounter) Touch(name string) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	if _, ok := dc.v[name]; !ok {
		dc.Reset(name)
	}
}

func (dc *LeaseDelegatedCounter) Counter(name string) int {
	dc.lock.RLock()
	defer dc.lock.Unlock()

	return dc.v[name]
}

func onLeaseCreate(n interface{}) {
	nl := n.(*coordv1.Lease)
	//klog.Infof("new lease: %v\n", nl)
	ldc.Reset(nl.Name)

	if val, ok := nl.Annotations[constant.DelegateHeartBeat]; ok {
		if val == "true" {
			ldc.Inc(nl.Name)
		}
	}
}

func onLeaseUpdate(o interface{}, n interface{}) {
	//ol := o.(*coordv1.Lease)
	nl := n.(*coordv1.Lease)
	//klog.Infof("updated lease: %v\n", nl)

	ldc.Touch(nl.Name)

	//oval, ook := ol.Annotations[constant.DelegateHeartBeat]
	nval, nok := nl.Annotations[constant.DelegateHeartBeat]

	if nok && nval == "true" {
		ldc.Inc(nl.Name)
		if ldc.Counter(nl.Name) >= constant.LeaseDelegationThreshold {
			GetController().taintNodeNotSchedulable(nl.Name)
		}
	} else {
		if ldc.Counter(nl.Name) >= constant.LeaseDelegationThreshold {
			GetController().deTaintNodeNotSchedulable(nl.Name)
		}
		ldc.Reset(nl.Name)
	}
}

func onNodeCreate(n interface{}) {
	node := n.(*corev1.Node)
	pool, ok := utils.NodeNodepool(node)
	if ok {
		GetController().nodepoolMap.Add(pool, node.Name)
	}
}

func onNodeDelete(n interface{}) {
	node := n.(*corev1.Node)
	pool, ok := utils.NodeNodepool(node)
	if ok {
		GetController().nodepoolMap.Del(pool, node.Name)
	}
}

func onNodeUpdate(o interface{}, n interface{}) {
	on := o.(*corev1.Node)
	nn := n.(*corev1.Node)
	opool, ook := utils.NodeNodepool(on)
	npool, nok := utils.NodeNodepool(nn)
	if !ook && !nok {
		return
	}
	if !ook && nok {
		GetController().nodepoolMap.Add(npool, nn.Name)
	} else if ook && !nok {
		GetController().nodepoolMap.Del(opool, on.Name)
	} else {
		if opool != npool {
			GetController().nodepoolMap.Del(opool, on.Name)
			GetController().nodepoolMap.Add(npool, nn.Name)
		}
	}
}

func GetController() *Controller {
	if ctl == nil {
		ctl = &Controller{
			client: client.GetClientFromCluster(),
			//client: client.GetClientFromEnv(os.Getenv("HOME") + "/.kube/config"),
		}
	}

	return ctl
}

func (nc *Controller) taintNodeNotSchedulable(name string) {
	node, err := nc.nodeLister.Get(name)
	if err != nil {
		klog.Error(err)
		return
	}
	taints := node.Spec.Taints
	if utils.TaintKeyExists(taints, constant.NodeNotSchedulableTaint) {
		return
	}
	nn := node.DeepCopy()
	t := corev1.Taint{
		Key:    constant.NodeNotSchedulableTaint,
		Value:  "true",
		Effect: corev1.TaintEffectNoSchedule,
	}
	nn.Spec.Taints = append(nn.Spec.Taints, t)
	nn, err = nc.client.CoreV1().Nodes().Update(context.TODO(), nn, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
	}
}

func (nc *Controller) deTaintNodeNotSchedulable(name string) {
	node, err := nc.nodeLister.Get(name)
	if err != nil {
		klog.Error(err)
		return
	}
	taints := node.Spec.Taints
	taints, deleted := utils.DeleteTaintsByKey(taints, constant.NodeNotSchedulableTaint)
	if !deleted {
		return
	}
	nn := node.DeepCopy()
	nn.Spec.Taints = taints
	nn, err = nc.client.CoreV1().Nodes().Update(context.TODO(), nn, metav1.UpdateOptions{})
	if err != nil {
		klog.Error(err)
	}
}

func (nc *Controller) Run() {
	stopCH := make(chan (struct{}))
	stopper := make(chan (struct{}))
	defer close(stopper)
	ldc = &LeaseDelegatedCounter{
		v: make(map[string]int),
	}

	klog.Info("create lease lister")
	nc.leaseLister = lister.CreateLeaseLister(nc.client, stopper, onLeaseCreate, onLeaseUpdate, nil)
	klog.Info("create node lister")
	nc.nodeLister = lister.CreateNodeLister(nc.client, stopper, onNodeCreate, onNodeUpdate, onNodeDelete)
	klog.Info("create nodepool map")
	nc.nodepoolMap = utils.NewNodepoolMap()
	nl, err := nc.nodeLister.List(labels.Everything())
	if err != nil {
		klog.Error(err)
	}
	nc.nodepoolMap.Sync(nl)
	klog.Info("create webhook")
	go webhook.Run(nc.nodeLister, nc.leaseLister, nc.nodepoolMap)
	<-stopCH
}
