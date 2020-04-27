PROG=rke2
GOLANGCI_VERSION=v1.25.1
REPO ?= rancher
IMAGE=${REPO}/rke2-runtime
PKG=github.com/rancher/rke2

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

VERSION=$(shell git describe --match 'v[0-9]*' --dirty='.dirty' --always --tags)
REVISION=$(shell git rev-parse HEAD)$(shell if ! git diff --no-ext-diff --quiet --exit-code; then echo .dirty; fi)
RELEASE=${PROG}-$(VERSION).${GOOS}-${GOARCH}

ifdef BUILDTAGS
    GO_BUILDTAGS = ${BUILDTAGS}
endif
# Build tags seccomp, apparmor and selinux are needed by CRI plugin.
GO_BUILDTAGS ?= todo
GO_BUILDTAGS += ${DEBUG_TAGS}
GO_TAGS=$(if $(GO_BUILDTAGS),-tags "$(GO_BUILDTAGS)",)
GO_LDFLAGS=-ldflags '-X $(PKG)/version.Version=$(VERSION) -X $(PKG)/version.Revision=$(REVISION) -X $(PKG)/version.Package=$(PACKAGE) $(EXTRA_LDFLAGS)'


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
	docker build -t ${REPO}/rke2-runtime:${VERSION} .

bin/golangci-lint:
	curl -sL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s ${GOLANGCI_VERSION}

validate:                                ## Run go fmt/vet
	go fmt ./...
	go vet ./...

validate-ci: validate bin/golangci-lint  ## Run more validation for CI
	./bin/golangci-lint run

run: build-debug
	./bin/${PROG} server

bin/dlv:
	go build -o bin/dlv github.com/go-delve/delve/cmd/dlv

remote-debug: build-debug bin/dlv        ## Run with remote debugging listening on :2345
	./bin/dlv --listen=:2345 --headless=true --api-version=2 --accept-multiclient exec ./bin/${PROG} server

.dev-shell-build:
	docker build -t ${PROG}-dev --target shell -f Dockerfile --target dapper .

clean-cache:                             ## Clean up docker base caches used for development
	docker rm -fv ${PROG}-dev-shell
	docker volume rm ${PROG}-cache ${PROG}-pkg

clean:                                   ## Clean up workspace
	rm -rf bin dist

dev-shell: .dev-shell-build              ## Launch a development shell to run test builds
	docker run --rm --name ${PROG}-dev-shell -ti -v $${HOME}:$${HOME} -v ${PROG} -w $$(pwd) --privileged --net=host -v ${PROG}-pkg:/go/pkg -v ${PROG}-cache:/root/.cache/go-build ${PROG}-dev bash

dev-shell-enter:                         ## Enter the development shell on another terminal
	docker exec -it ${PROG}-dev-shell bash

artifacts: build
	mkdir -p dist/artifacts
	cp bin/${PROG} dist/artifacts/${RELEASE}

.ci: validate-ci artifacts

in-docker-%: .dapper                     ## Advanced: wraps any target in Docker environment, for example: in-docker-build-debug
	mkdir -p bin/ dist/
	./.dapper -f Dockerfile --target dapper make $*

./.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/v0.5.0/dapper-$$(uname -s)-$$(uname -m) > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
