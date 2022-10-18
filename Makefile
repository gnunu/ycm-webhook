DOCKER_IMG=pod-coordinator-webhook:latest
DOCKER_HUB=aibox03.bj.intel.com:5000

.PHONY: test
test:
	@echo "\n🛠️  Running unit tests..."
	go test ./... -v

.PHONY: build
build:
	@echo "\n🔧  Building Go binaries..."
	GOOS=linux GOARCH=amd64 go build -o bin/pod-coordinator .

.PHONY: image
image:
	@echo "\n📦 Building pod-coordinator-webhook Docker image..."
	DOCKER_BUILDKIT=1 docker build --build-arg http_proxy=${http_proxy} --build-arg https_proxy=${https_proxy} -t ${DOCKER_IMG} .

.PHONY: push
push:
	@echo "\n📦 Pushing pod-coordinator-webhook image..."
	docker tag ${DOCKER_IMG} ${DOCKER_HUB}/${DOCKER_IMG}
	docker push ${DOCKER_HUB}/${DOCKER_IMG}

.PHONY: manifest
manifest:
	@echo "\n📦 generate deployment manifest..."
	helm template pod-coordinator charts/pod-coordinator > config/pod-coordinator.yaml

.PHONY: deploy-secret
deploy-secret:
	@echo "\n⚙️  Deploying secret..."
	./webhook-create-signed-cert.sh --service pod-coordinator-webhook --namespace kube-system --secret pod-coordinator-webhook-tls

.PHONY: deploy-config
deploy-config:
	@echo "\n⚙️  Applying cluster config..."
	kubectl apply -f dev/manifests/cluster-config/

.PHONY: delete-config
delete-config:
	@echo "\n♻️  Deleting Kubernetes cluster config..."
	kubectl delete -f dev/manifests/cluster-config/

.PHONY: deploy
deploy: push delete deploy-config
	@echo "\n🚀 Deploying simple-kubernetes-webhook..."
	kubectl apply -f dev/manifests/webhook/

.PHONY: delete
delete:
	@echo "\n♻️  Deleting simple-kubernetes-webhook deployment if existing..."
	kubectl delete -f dev/manifests/webhook/ || true

.PHONY: delete-all
delete-all: delete delete-config delete-pod delete-bad-pod
