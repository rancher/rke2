##========================= Terraform Tests =========================#
include ./config.mk


TAGNAME ?= default
tf-up:
	@cd ../.. && docker build . -q -f ./tests/terraform/scripts/Dockerfile.build -t rke2-tf-${TAGNAME}

.PHONY: tf-run
tf-run:
	@docker run -d --name rke2-tf-test-${IMGNAME} -t \
      -e AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" \
      -e AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" \
      -v ${ACCESS_KEY_LOCAL}:/go/src/github.com/rancher/rke2/tests/terraform/modules/config/.ssh/aws_key.pem \
       rke2-tf-${TAGNAME} sh -c 'cd ./tests/terraform ; \
       				  if [ -n "${ARGNAME}" ]; then \
                         go test -v -timeout=45m \
                           ./${TESTDIR}/... \
                           -"${ARGNAME}"="${ARGVALUE}"; \
                      elif [ -z "${TESTDIR}" ]; then \
                         go test -v -timeout=40m \
                           ./createcluster/...; \
                      else \
                         go test -v -timeout=45m \
                           ./${TESTDIR}/...; \
                      fi'

.PHONY: tf-logs
tf-logs:
	@docker logs -f rke2-tf-test-${IMGNAME}

.PHONY: tf-down
tf-down:
	@echo "Removing containers and images"
	@docker stop $$(docker ps -a -q --filter="name=rke2-tf*")
	@docker rm $$(docker ps -a -q --filter="name=rke2-tf*")
	@docker rmi $$(docker images -q --filter="reference=rke2-tf*")

.PHONY: tf-clean
tf-clean:
	@./scripts/delete_resources.sh

.PHONY: tf-complete
tf-tests-complete: tf-clean tf-down tf-remove-state tf-up tf-run


#========================= Run terraform tests locally =========================#

.PHONY: tf-create
tf-create:
	@go test -timeout=45m -v ./createcluster/...

.PHONY: tf-upgrade
tf-upgrade:
	@go test -timeout=45m -v ./upgradecluster/... -${ARGNAME}=${ARGVALUE}

.PHONY: tf-remove-state
tf-remove-state:
	@rm -rf ./modules/.terraform
	@rm -rf ./modules/.terraform.lock.hcl ./modules/terraform.tfstate ./modules/terraform.tfstate.backup

.PHONY: tf-test-suite
tf-test-suite:
	@make tf-remove-state && make tf-create ; sleep 5 && \
	  make tf-remove-state && make tf-upgrade ${ARGNAME}=${ARGVALUE}

.PHONY: tf-test-suite-same-cluster
tf-test-suite-same-cluster:
	@make tf-create ; sleep 5 && make tf-upgrade ${ARGNAME}=${ARGVALUE}

#========================= TestCode Static Quality Check =========================#
.PHONY: vet-lint                      ## Run locally only inside Tests package
vet-lint:
	@echo "Running go vet and lint"
	@go vet ./${TESTDIR} && golangci-lint run --tests