
LOCAL_BIN := $(CURDIR)/bin
CONTROLLER_GEN := $(LOCAL_BIN)/controller-gen
CONTROLLER_TOOLS_VERSION := v0.19.0

.PHONY: all generate manifests fmt vet test run build docker-build clean

all: fmt vet test build

$(CONTROLLER_GEN):
	mkdir -p $(LOCAL_BIN)
	GOBIN=$(LOCAL_BIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

generate: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object:headerFile="" paths="./..."

manifests: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) rbac:roleName=manager-role \
		paths="./..." \
		output:rbac:artifacts:config=config/rbac

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

.PHONY: run
# Use RUN_ARGS to pass arguments to the manager, e.g. RUN_ARGS="--webhook-port=-1".
RUN_ARGS="--debug"
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go $(RUN_ARGS)

build:
	mkdir -p bin
	go build -o bin/manager ./cmd/manager

docker-build:
	docker build -t spiffe-pod-certificate-controller:dev .
