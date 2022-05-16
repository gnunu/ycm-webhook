DOCKER_IMG=aibox03.bj.intel.com:5000/ycm-webhook:latest

.PHONY: test
test:
	@echo "\n🛠️  Running unit tests..."
	go test ./...

.PHONY: build
build:
	@echo "\n🔧  Building Go binaries..."
	GOOS=linux GOARCH=amd64 go build -o bin/ycm-webhook-linux-amd64 .

.PHONY: image
image:
	@echo "\n📦 Building ycm-webhook Docker image..."
	DOCKER_BUILDKIT=1 docker build --build-arg http_proxy=${http_proxy} --build-arg https_proxy=${https_proxy} -t ${DOCKER_IMG} .

.PHONY: push
push: image
	@echo "\n📦 Pushing ycm-webhook image into Kind's Docker daemon..."
	docker push ${DOCKER_IMG}

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
