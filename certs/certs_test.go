package certs

import (
	"fmt"
	"testing"
)

var (
	ns string = "kube-system"
	sn string = "ycm-webhook"
)

func TestGenerateCerts(t *testing.T) {
	certs := GenerateCerts(ns, sn)
	fmt.Println(certs)
}

func TestServiceToCommonName(t *testing.T) {
	cn := ServiceToCommonName(ns, sn)
	if cn != "ycm-webhook.kube-system.svc" {
		t.Errorf("got nil, wanted not nil")
	} else {
		fmt.Println(cn)
	}
}
