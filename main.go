package main

import poolcoordinator "github.com/openyurtio/openyurt/pkg/controller/poolcoordinator"

func main() {
	nc := poolcoordinator.GetController()
	nc.Run()
}
