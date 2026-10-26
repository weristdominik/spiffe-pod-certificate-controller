LOCAL_BIN := $(CURDIR)/bin
CONTROLLER_GEN := $(LOCAL_BIN)/controller-gen
CONTROLLER_TOOLS_VERSION := v0.19.0

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

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

.PHONY: build
build:  ## Compile go code.
	mkdir -p bin
	go build -o bin/manager ./cmd/manager

docker-build:
	docker build -t spiffe-pod-certificate-controller:dev .
