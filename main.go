package main

import poolcoordinator "github.com/openyurtio/openyurt/pkg/controller/poolcoordinator/controller"

func main() {
	nc := poolcoordinator.GetController()
	nc.Run()
}
