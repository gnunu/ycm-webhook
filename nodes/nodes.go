package nodes

import (
	"context"

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

/// return nodepool the node is in
func GetNodePoolName(node *corev1.Node) string {
	if node.Labels != nil {
		return node.Labels[LabelKeyNodePool]
	}
	return ""
}

/// return nodepools in the cluster
func getNodePools() []string {
	return nil
}

/// if nodepool is deployed pool-coordinator
func NodePoolIsInAutonomy(name string) bool {
	return false
}

func GetNode(clientset *kubernetes.Clientset, name string) *corev1.Node {
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), name, v1.GetOptions{})
	if err != nil {
		klog.Error(err)
		return nil
	}
	return node
}

func GetNodeStatus(clientset *kubernetes.Clientset, name string) {
}
