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

.PHONY: build-debug
build-debug:                             ## Debug build using host go tools
	GODEBUG=y ./scripts/build

.PHONY: build-images
build-images:                             ## Build all images and image tarballs (including airgap)
	./scripts/build-images

.PHONY: build-image-kubernetes
build-image-kubernetes:                               ## Build the kubernetes image
	./scripts/build-image-kubernetes

.PHONY: build-image-runtime
build-image-runtime: build-charts					## Build the runtime image
	./scripts/build-image-runtime

.PHONY: publish-image-kubernetes
publish-image-kubernetes: build-image-kubernetes
	./scripts/publish-image-kubernetes

.PHONY: publish-image-runtime
publish-image-runtime: build-image-runtime
	./scripts/publish-image-runtime

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
dev-shell-build: build
	./scripts/dev-shell-build

.PHONY: clean-cache
clean-cache:                             ## Clean up docker base caches used for development
	./scripts/clean-cache

.PHONY: clean
clean:                                   ## Clean up workspace
	./scripts/clean

.PHONY: dev-shell
dev-shell: dev-shell-build              ## Launch a development shell to run test builds
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

.PHONY: publish-manifest-runtime
publish-manifest-runtime: build-image-runtime					## Create and push the rke2-runtime manifest
	./scripts/publish-manifest-runtime

.PHONY: publish-manifest-kubernetes
publish-manifest-kubernetes: build-image-kubernetes						## Create and push the kubernetes manifest
	./scripts/publish-manifest-kubernetes

.PHONY: dispatch
dispatch:								## Send dispatch event to rke2-upgrade repo
	./scripts/dispatch

.PHONY: build-charts
build-charts: 						## Download packaged helm charts
	./scripts/build-charts

.PHONY: package
package: build 						## Package the rke2 binary
	./scripts/package

.PHONY: package-images
package-images: build-images		## Package docker images for airgap environment
	./scripts/package-images

.PHONY: package-bundle
package-bundle: build						## Package the tarball bundle
	./scripts/package-bundle

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
