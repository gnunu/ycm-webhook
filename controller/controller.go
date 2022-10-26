package controller

import (
	"context"
	"fmt"

	"github.com/openyurtio/pkg/controller/poolcoordinator/client"
	"github.com/openyurtio/pkg/controller/poolcoordinator/constant"
	"github.com/openyurtio/pkg/controller/poolcoordinator/lister"
	"github.com/openyurtio/pkg/controller/poolcoordinator/utils"
	"github.com/openyurtio/pkg/controller/poolcoordinator/webhook"
	coordv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	leaselisterv1 "k8s.io/client-go/listers/coordination/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog/v2"
)

type Controller struct {
	client      *kubernetes.Clientset
	nodeLister  listerv1.NodeLister
	leaseLister leaselisterv1.LeaseNamespaceLister
}

var ctl *Controller

func onLeaseUpdate(o interface{}, n interface{}) {
	//ol := o.(*coordv1.Lease)
	nl := n.(*coordv1.Lease)

	if nl.Annotations != nil {
		if nl.Annotations[constant.DelegateHeartBeat] == "true" {
			GetController().taintNodeNotSchedulable(nl.Name)
		} else {
			GetController().deTaintNodeNotSchedulable(nl.Name)
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
	klog.Info("create lease lister")
	nc.leaseLister = lister.CreateLeaseLister(nc.client, stopper, nil, onLeaseUpdate, nil)
	klog.Info("create node lister")
	nc.nodeLister = lister.CreateNodeLister(nc.client, stopper, nil, nil, nil)
	klog.Info("create webhook")
	go webhook.Run(nc.nodeLister, nc.leaseLister)
	n, _ := nc.nodeLister.Get("ai-ice-vm31")
	fmt.Printf("%v\n", n)
	<-stopCH
}
