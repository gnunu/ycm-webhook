package main

import (
	"github.com/openyurtio/pkg/controller/poolcoordinator/controller"
)

func main() {
	nc := controller.GetController()
	nc.Run()
}
