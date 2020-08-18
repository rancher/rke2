.PHONY: default
default: in-docker-build                 ## Build using docker environment (default target)
	@echo "Run make help for info about other make targets"

.PHONY: ci
ci: in-docker-.ci                        ## Run CI locally

.PHONY: ci-shell
ci-shell: clean .dapper                  ## Launch a shell in the CI environment to troubleshoot. Runs clean first
	@echo
	@echo '######################################################'
	@echo '# Run "make dapper-ci" to reproduce CI in this shell #'
	@echo '######################################################'
	@echo
	./.dapper -f Dockerfile --target dapper -s

.PHONY: dapper-ci
dapper-ci: .ci                           ## Used by Drone CI, does the same as "ci" but in a Drone way

.ci: validate build package

.PHONY: build
build:                                   ## Build using host go tools
	./scripts/build

.PHONY: build-airgap
build-airgap: | build image k8s-image build/images/airgap.tar	## Build all images for an airgapped installation

.PHONY: build-debug
build-debug:                             ## Debug build using host go tools
	./scripts/build-debug

.PHONY: image
image: download-charts					## Build final docker image for push
	./scripts/image

.PHONY: k8s-image
k8s-image:                               ## Build final docker image for kubernetes
	./scripts/k8s-image

.PHONY: image-publish
image-publish: image
	./scripts/image-publish

.PHONY: validate
validate:                                ## Run go fmt/vet
	./scripts/validate

.PHONY: run
run: build-debug
	./scripts/run

.PHONY: remote-debug
remote-debug: build-debug        		 ## Run with remote debugging listening on :2345
	./scripts/remote-debug

.PHONY: remote-debug-exit
remote-debug-exit:              		 ## Kill dlv started with make remote-debug
	./scripts/remote-debug-exit

.PHONY: dev-shell-build
dev-shell-build: build-airgap
	./scripts/dev-shell-build

build/images/airgap.tar:
	./scripts/airgap-images.sh

.PHONY: clean-cache
clean-cache:                             ## Clean up docker base caches used for development
	./scripts/clean-cache

.PHONY: clean
clean:                                   ## Clean up workspace
	./scripts/clean

.PHONY: dev-shell
dev-shell: k8s-image download-charts image dev-shell-build              ## Launch a development shell to run test builds
	./scripts/dev-shell

.PHONY: dev-shell-enter
dev-shell-enter:                       ## Enter the development shell on another terminal
	./scripts/dev-shell-enter

.PHONY: dev-peer
dev-peer: dev-shell-build              ## Launch a server peer to run test builds
	./scripts/dev-peer

.PHONY: dev-peer-enter
dev-peer-enter:                         ## Enter the peer shell on another terminal
	./scripts/dev-peer-enter

.PHONY: k8s-image-publish
k8s-image-publish: k8s-image
	./scripts/k8s-image-publish

.PHONY: image-manifest
image-manifest:							## mainfest rke2-runtime image
	./scripts/image-manifest

.PHONY: dispatch
dispatch:								## Send dispatch event to rke2-upgrade repo
	./scripts/dispatch

.PHONY: download-charts
download-charts: 						## Download packaged helm charts
	./scripts/download-charts

.PHONY: package
package: download-charts package-airgap ## Package the rke2 binary
	./scripts/package

.PHONY: package-airgap
package-airgap: build/images/airgap.tar		## Package docker images for airgap environment
	./scripts/package-airgap

./.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/v0.5.0/dapper-$$(uname -s)-$$(uname -m) > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

in-docker-%: .dapper                     ## Advanced: wraps any target in Docker environment, for example: in-docker-build-debug
	mkdir -p ./bin/ ./dist/ ./build
	./.dapper -f Dockerfile --target dapper make $*

.PHONY: help
help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
