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

.ci: validate build package

.PHONY: build
build:                                   ## Build using host go tools
	./scripts/build

build-debug:                             ## Debug build using host go tools
	./scripts/build-debug

image:                                   ## Build final docker image for push
	./scripts/image

k8s-image:                               ## Build final docker image for kubernetes
	./scripts/k8s-image

image-publish: image
	./scripts/image-publish

validate:                                ## Run go fmt/vet
	./scripts/validate

run: build-debug
	./scripts/run

remote-debug: build-debug        		 ## Run with remote debugging listening on :2345
	./scripts/remote-debug

remote-debug-exit:              		 ## Kill dlv started with make remote-debug
	./scripts/remote-debug-exit

dev-shell-build: build/images/airgap.tar
	./scripts/dev-shell-build

build/images/airgap.tar:
	./scripts/airgap-images.sh

clean-cache:                             ## Clean up docker base caches used for development
	./scripts/clean-cache

clean:                                   ## Clean up workspace
	./scripts/clean

dev-shell: image k8s-image dev-shell-build              ## Launch a development shell to run test builds
	./scripts/dev-shell

dev-shell-enter:                       ## Enter the development shell on another terminal
	./scripts/dev-shell-enter

dev-peer: dev-shell-build              ## Launch a server peer to run test builds
	./scripts/dev-peer

dev-peer-enter:                         ## Enter the peer shell on another terminal
	./scripts/dev-peer-enter

k8s-image-publish: k8s-image
	./scripts/k8s-image-publish

image-manifest:							## mainfest rke2-runtime image
	./scripts/image-manifest

dispatch:								## Send dispatch event to rke2-upgrade repo
	./scripts/dispatch

download-charts: 						## Download packaged helm charts
	./scripts/download-charts

package: download-charts package-airgap ## Package the rke2 binary
	./scripts/package

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

help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
