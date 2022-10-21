package nodes

import (
	"context"
	"time"

	"github.com/openyurtio/pkg/webhooks/pod-validator/lister"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	AnnotationKeyNodeAutonomy string = "node.beta.openyurt.io/autonomy" // nodeutil.AnnotationKeyNodeAutonomy
	LabelKeyNodePool          string = "apps.openyurt.io/nodepool"
)

func NodeIsInAutonomy(node *corev1.Node) bool {
	if node.Annotations != nil && node.Annotations[AnnotationKeyNodeAutonomy] == "true" {
		return true
	}
	return false
}

func NodeIsAlive(node *corev1.Node) bool {
	lease, err := lister.LeaseLister().Get(node.Name)
	if err != nil {
		klog.Error(err)
		return false
	}
	diff := time.Now().Sub(lease.GetCreationTimestamp().Time)
	if diff.Seconds() > 40 {
		return false
	}
	return true
}

/// return nodepool the node is in
func GetNodePoolName(node *corev1.Node) string {
	if node.Labels != nil {
		return node.Labels[LabelKeyNodePool]
	}
	return ""
}

func GetNode(clientset *kubernetes.Clientset, name string) *corev1.Node {
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), name, v1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return nil
	}
	return node
}
