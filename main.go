package main

import "github.com/openyurtio/pkg/webhooks/pod-validator/lister"

func main() {
	lister.CreateListers()
	RegisterWebhook()
}
