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

.PHONY: build-binary
build-binary:                             	## Build only the Linux binary using host go tools
	./scripts/build-binary

.PHONY: build-windows-binary
build-windows-binary:                       ## Build only the Windows binary using host go tools
	./scripts/build-windows-binary

.PHONY: build-debug
build-debug:                             ## Debug build using host go tools
	GODEBUG=y ./scripts/build-binary

.PHONY: scan-images
scan-images:
	./scripts/scan-images

.PHONY: build-images
build-images:                             ## Build all images and image tarballs (including airgap)
	./scripts/build-images

.PHONY: build-windows-images
build-windows-images:                     ## Build only the Windows images and tarballs (including airgap)
	./scripts/build-windows-images

.PHONY: build-image-kubernetes
build-image-kubernetes:                   ## Build the kubernetes image
	./scripts/build-image-kubernetes

.PHONY: build-image-runtime
build-image-runtime:                      ## Build the runtime image
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

.PHONY: validate-release
validate-release: 
	./scripts/validate-release

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
dev-shell-build: in-docker-build
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

.PHONY: publish-manifest-kubernetes
publish-manifest-kubernetes: build-image-kubernetes						## Create and push the kubernetes manifest
	./scripts/publish-manifest-kubernetes

.PHONY: dispatch
dispatch:								## Send dispatch event to rke2-upgrade repo
	./scripts/dispatch

.PHONY: package
package: build 						## Package the rke2 binary
	./scripts/package

.PHONY: package-images
package-images: build-images		## Package docker images for airgap environment
	./scripts/package-images

.PHONY: package-windows-images
package-windows-images: build-windows-images		## Package Windows crane images for airgap environment
	./scripts/package-windows-images

.PHONY: package-bundle
package-bundle: build-binary					## Package the tarball bundle
	./scripts/package-bundle

.PHONY: package-windows-bundle
package-windows-bundle: build-windows-binary	## Package the Windows tarball bundle
	./scripts/package-windows-bundle

.PHONY: test
test: unit-tests integration-tests

.PHONY: unit-tests
unit-tests:
	./scripts/unit-tests

.PHONY: integration-tests
integration-tests:
	./scripts/test

./.dapper:
	@echo Downloading dapper
	@curl -sL https://releases.rancher.com/dapper/v0.5.8/dapper-$$(uname -s)-$$(uname -m) > .dapper.tmp
	@@chmod +x .dapper.tmp
	@./.dapper.tmp -v
	@mv .dapper.tmp .dapper

in-docker-%: .dapper                     ## Advanced: wraps any target in Docker environment, for example: in-docker-build-debug
	mkdir -p ./bin/ ./dist/ ./build
	./.dapper -f Dockerfile --target dapper make $*

.PHONY: help
help: ## this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

serve-docs: mkdocs
	docker run -p 8000:8000 --rm -it -v $${PWD}:/docs mkdocs serve -a 0.0.0.0:8000

mkdocs:
	docker build -t mkdocs -f Dockerfile.docs .

##========================= Terraform Tests =========================#
include ./config.mk

tf-tests-up:
	@docker build . -q -f ./tests/terraform/scripts/Dockerfile.build -t rke2-tf

.PHONY: tf-tests-run
tf-tests-run:
	@docker run -d --rm --name rke2-tf-test${IMGNAME} -t \
      -e AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" \
      -e AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" \
      -v ${ACCESS_KEY_LOCAL}:/go/src/github.com/rancher/rke2/tests/terraform/modules/config/.ssh/aws_key.pem \
      rke2-tf sh -c 'if [ -n "${ARGNAME}" ]; then \
                         go test -v -timeout=45m \
                           ./tests/${TESTDIR}/... \
                           -"${ARGNAME}"="${ARGVALUE}"; \
                       elif [ -z "${TESTDIR}" ]; then \
                         go test -v -timeout=40m \
                           ./tests/terraform/createcluster/...; \
                       else \
                         go test -v -timeout=45m \
                           ./tests/${TESTDIR}/...; \
                       fi'

.PHONY: tf-tests-logs
tf-tests-logs:
	@docker logs -f rke2-tf-test${IMGNAME}

.PHONY: tf-tests-down
tf-tests-down:
	@echo "Removing containers and images"
	@docker stop $$(docker ps -a -q --filter="name=rke2-tf*")
	@docker rm $$(docker ps -a -q --filter="name=rke2-tf*")

tf-tests-clean:
	@./tests/terraform/scripts/delete_resources.sh

.PHONY: tf-tests
tf-tests: tf-tests-clean tf-tests-down tf-tests-up tf-tests-run


#========================= Run terraform tests locally =========================#
.PHONY: tf-tests-local-createcluster
tf-tests-local-createcluster:
	@go test -timeout=40m -v ./tests/terraform/createcluster/...

.PHONY: tf-tests-local-upgradecluster
tf-tests-local-upgradecluster:
	@go test -timeout=45m -v ./tests/terraform/upgradecluster/... -${ARGVALUE}=${ARGNAME}

#========================= TestCode Static Quality Check =========================#
.PHONY: tf-tests-logs                     ## Run locally only inside Tests package
vet-lint:
	@echo "Running go vet and lint"
	@go vet ./tests/${TESTDIR}
	@cd tests/${TESTDIR} && golangci-lint run --tests
