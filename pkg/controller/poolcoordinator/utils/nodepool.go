package utils

import (
	"sync"
	"time"

	"github.com/openyurtio/openyurt/pkg/controller/poolcoordinator/constant"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	leaselisterv1 "k8s.io/client-go/listers/coordination/v1"
	"k8s.io/klog/v2"
)

type NodepoolMap struct {
	nodepools map[string]sets.String
	lock      sync.Mutex
}

func NewNodepoolMap() *NodepoolMap {
	return &NodepoolMap{
		nodepools: make(map[string]sets.String),
	}
}

func (m *NodepoolMap) Add(pool, node string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.nodepools[pool] == nil {
		m.nodepools[pool] = sets.String{}
	}
	m.nodepools[pool].Insert(node)
}

func (m *NodepoolMap) Del(pool, node string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.nodepools[pool] == nil {
		return
	}

	m.nodepools[pool].Delete(node)
	if m.nodepools[pool].Len() == 0 {
		delete(m.nodepools, pool)
	}
}

func (m *NodepoolMap) Count(pool string) int {
	if m.nodepools[pool] != nil {
		return m.nodepools[pool].Len()
	}
	return 0
}

func (m *NodepoolMap) Nodes(pool string) []string {
	if m.nodepools[pool] != nil {
		return m.nodepools[pool].UnsortedList()
	}
	return []string{}
}

func (m *NodepoolMap) Sync(nodes []*corev1.Node) {
	for _, n := range nodes {
		pool, ok := NodeNodepool(n)
		if ok {
			m.Add(pool, n.Name)
		}
	}
}

func NodeIsInAutonomy(node *corev1.Node) bool {
	if node.Annotations != nil && node.Annotations[constant.AnnotationKeyNodeAutonomy] == "true" {
		return true
	}
	return false
}

func NodeIsAlive(leaseLister leaselisterv1.LeaseNamespaceLister, nodeName string) bool {
	lease, err := leaseLister.Get(nodeName)
	if err != nil {
		klog.Error(err)
		return false
	}
	diff := time.Now().Sub(lease.Spec.RenewTime.Time)
	if diff.Seconds() > 40 {
		return false
	}
	return true
}

func CountAliveNode(leaseLister leaselisterv1.LeaseNamespaceLister, nodes []string) int {
	cnt := 0
	for _, n := range nodes {
		if NodeIsAlive(leaseLister, n) {
			cnt++
		}
	}
	return cnt
}

func NodeNodepool(node *corev1.Node) (string, bool) {
	if node.Labels != nil {
		val, ok := node.Labels[constant.LabelKeyNodePool]
		return val, ok
	}

	return "", false
}
