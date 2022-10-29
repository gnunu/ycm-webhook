package utils

import (
	"reflect"
	"testing"
)

func TestNodeSet(t *testing.T) {
	ns := &NodeSet{}
	ns.Add("node1")
	ns.Add("node2")
	nodes := ns.Nodes()
	expected := []string{"node1", "node2"}
	if !reflect.DeepEqual(nodes, expected) {
		t.Errorf("expect %v, but %v returned", expected, nodes)
	}
}

func TestNodeMap(t *testing.T) {
	nm := NewNodepoolMap()
	nm.Add("pool1", "node1")
	nm.Add("pool1", "node2")
	nm.Add("pool2", "node3")
	nm.Add("pool2", "node4")
	nm.Add("pool2", "node5")

	if nm.Count("pool1") != 2 {
		t.Errorf("expect %v, but %v returned", 2, nm.Count("pool1"))
	}
	if nm.Count("pool2") != 3 {
		t.Errorf("expect %v, but %v returned", 3, nm.Count("pool2"))
	}
	nm.Del("pool2", "node4")
	if nm.Count("pool2") != 2 {
		t.Errorf("expect %v, but %v returned", 2, nm.Count("pool2"))
	}
	nodes := nm.Nodes("pool2")
	expected := []string{"node3", "node5"}
	if !reflect.DeepEqual(nodes, expected) {
		t.Errorf("expect %v, but %v returned", expected, nodes)
	}

	nm.Del("pool1", "node1")
	nm.Del("pool1", "node2")
	if nm.Count("pool1") != 0 {
		t.Errorf("expect %v, but %v returned", 0, nm.Count("pool1"))
	}
	nodes = nm.Nodes("pool1")
	expected = []string{}
	if !reflect.DeepEqual(nodes, expected) {
		t.Errorf("expect %v, but %v returned", expected, nodes)
	}
}
