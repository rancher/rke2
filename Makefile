PROG=rke2
GOLANGCI_VERSION=v1.27.0
REPO ?= rancher
IMAGE=${REPO}/rke2-runtime
K3S_PKG=github.com/rancher/k3s

ifneq "$(strip $(shell command -v go 2>/dev/null))" ""
	GOOS ?= $(shell go env GOOS)
	GOARCH ?= $(shell go env GOARCH)
else
	ifeq ($(GOOS),)
		# approximate GOOS for the platform if we don't have Go and GOOS isn't
		# set. We leave GOARCH unset, so that may need to be fixed.
		ifeq ($(OS),Windows_NT)
			GOOS = windows
		else
			UNAME_S := $(shell uname -s)
			ifeq ($(UNAME_S),Linux)
				GOOS = linux
			endif
			ifeq ($(UNAME_S),Darwin)
				GOOS = darwin
			endif
			ifeq ($(UNAME_S),FreeBSD)
				GOOS = freebsd
			endif
		endif
	else
		GOOS ?= $$GOOS
		GOARCH ?= $$GOARCH
	endif
endif

ifndef GODEBUG
	EXTRA_LDFLAGS += -s -w
	DEBUG_GO_GCFLAGS :=
	DEBUG_TAGS :=
else
	DEBUG_GO_GCFLAGS := -gcflags=all="-N -l"
endif

ifdef DRONE_TAG
	VERSION=$(shell echo ${DRONE_TAG} | sed -e 's/+/-/g')
else
	VERSION=dev
endif
REVISION=$(shell git rev-parse HEAD)$(shell if ! git diff --no-ext-diff --quiet --exit-code; then echo .dirty; fi)
RELEASE=${PROG}.${GOOS}-${GOARCH}


BUILDTAGS     = netgo
GO_BUILDTAGS += ${BUILDTAGS}
GO_BUILDTAGS ?= no_embedded_executor
GO_BUILDTAGS += ${DEBUG_TAGS}
GO_TAGS=$(if $(GO_BUILDTAGS),-tags "$(GO_BUILDTAGS)",)
GO_LDFLAGS=-ldflags '-extldflags "-static"                        \
					 -X $(K3S_PKG)/pkg/version.Program=$(PROG)    \
					 -X $(K3S_PKG)/pkg/version.Version=$(VERSION) \
					 -X $(K3S_PKG)/pkg/version.Revision=$(REVISION) $(EXTRA_LDFLAGS)'


default: in-docker-build                 ## Build using docker environment (default target)
	@echo "Run make help for info about other make targets"

ci: in-docker-.ci                        ## Run CI locally

ci-shell: clean .dapper                  ## Launch a shell in the CI environment to troubleshoot. Runs clean first
	@echo
	@echo '######################################################'
	@echo '# Run "make dapper-ci" to reproduce CI in this shell #'
	@echo '######################################################'
	@echo
	./.dapper -f Dockerfile --target dapper -s

dapper-ci: .ci                           ## Used by Drone CI, does the same as "ci" but in a Drone way

build:                                   ## Build using host go tools
	go build ${DEBUG_GO_GCFLAGS} ${GO_GCFLAGS} ${GO_BUILD_FLAGS} -o bin/${PROG} ${GO_LDFLAGS} ${GO_TAGS}

build-debug:                             ## Debug build using host go tools
	$(MAKE) GODEBUG=y build

image:                                   ## Build final docker image for push
	docker build --build-arg KUBERNETES_VERSION=${VERSION} -t ${REPO}/rke2-runtime:${VERSION} .

bin/golangci-lint:
	curl -sL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s ${GOLANGCI_VERSION}

validate:                                ## Run go fmt/vet
	go fmt ./...
	go vet ./...

validate-ci: validate bin/golangci-lint  ## Run more validation for CI
	./bin/golangci-lint run

COMMAND ?= "server"
run: build-debug
	./bin/${PROG} ${COMMAND} ${ARGS}

bin/dlv:
	go build -o bin/dlv github.com/go-delve/delve/cmd/dlv

remote-debug: build-debug bin/dlv        ## Run with remote debugging listening on :2345
	CATTLE_DEV_MODE=true ./bin/dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec -- ./bin/${PROG} ${COMMAND} ${ARGS}

remote-debug-exit: bin/dlv               ## Kill dlv started with make remote-debug
	echo exit | ./bin/dlv connect :2345 --init /dev/stdin

.dev-shell-build:
	docker build -t ${PROG}-dev --target shell .

clean-cache:                             ## Clean up docker base caches used for development
	docker rm -fv ${PROG}-dev-shell
	docker volume rm ${PROG}-cache ${PROG}-pkg

clean:                                   ## Clean up workspace
	rm -rf bin dist build

dev-shell: .dev-shell-build              ## Launch a development shell to run test builds
	docker run --rm --name ${PROG}-dev-shell --hostname ${PROG}-server -ti -e WORKSPACE=$$(pwd) -p 127.0.0.1:2345:2345 -v $${HOME}:$${HOME} -v ${PROG} -w $$(pwd) --privileged -v ${PROG}-pkg:/go/pkg -v ${PROG}-cache:/root/.cache/go-build ${PROG}-dev bash

dev-shell-enter:                         ## Enter the development shell on another terminal
	docker exec -it ${PROG}-dev-shell bash

PEER ?= 1
dev-peer: .dev-shell-build              ## Launch a server peer to run test builds
	docker run --rm --link ${PROG}-dev-shell:${PROG}-server --name ${PROG}-peer${PEER} --hostname ${PROG}-peer${PEER} -p 127.0.0.1:234${PEER}:2345 -ti -e WORKSPACE=$$(pwd) -v $${HOME}:$${HOME} -v ${PROG} -w $$(pwd) --privileged -v ${PROG}-pkg:/go/pkg -v ${PROG}-cache:/root/.cache/go-build ${PROG}-dev bash

dev-peer-enter:                         ## Enter the peer shell on another terminal
	docker exec -it ${PROG}-peer${PEER} bash

artifacts: build download-charts
	mkdir -p dist/artifacts
	cp bin/${PROG} dist/artifacts/${RELEASE}

.ci: validate-ci artifacts

in-docker-%: .dapper                     ## Advanced: wraps any target in Docker environment, for example: in-docker-build-debug
	mkdir -p ./bin/ ./dist/
	./.dapper -f Dockerfile --target dapper make $*

CHARTS_DIR = build/static/charts
MANIFEST_DIR = manifests
CHARTS = canal:v3.13.3 coredns:1.10.101 kube-proxy:v1.18.4 metrics-server:2.11.100 nginx-ingress:1.36.300
download-charts:
	mkdir -p $(CHARTS_DIR)
	for chart in $(CHARTS); do \
    	  chart_name=`echo "$${chart}" | cut -d ":" -f 1`;\
    	  chart_version=`echo "$${chart}" | cut -d ":" -f 2`;\
    	  curl -sfL https://dev-charts.rancher.io/$$chart_name/$$chart_name-$$chart_version.tgz -o ${CHARTS_DIR}/$$chart_name-$$chart_version.tgz;\
    	  chart_content=`base64 -w 0 ${CHARTS_DIR}/$$chart_name-$$chart_version.tgz`;\
    	  sed -e "s|%{CHART_CONTENT}%|$$chart_content|g" $(MANIFEST_DIR)/$$chart_name.yml > $(CHARTS_DIR)/$$chart_name-chart.yml;\
    	  rm ${CHARTS_DIR}/$$chart_name-$$chart_version.tgz;\
    done

./.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/v0.5.0/dapper-$$(uname -s)-$$(uname -m) > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

k8s-image: k8s-image-build k8s-image-scan

k8s-image-build:
	docker build \
    	--build-arg TAG=${VERSION} -f Dockerfile.k8s -t ranchertest/kubernetes:${VERSION}-${GOARCH} .

SEVERITIES = HIGH,CRITICAL
k8s-image-scan:
	trivy --severity $(SEVERITIES) --no-progress --skip-update --ignore-unfixed ranchertest/kubernetes:${VERSION}-${GOARCH}

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
