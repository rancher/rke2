##========================= Terraform Tests =========================#
include ./config.mk

tf-tests-up:
	@cd ../ && docker build . -q -f ./tests/terraform/scripts/Dockerfile.build -t rke2-tf

.PHONY: tf-tests-run
tf-tests-run:
	@docker run -d --rm --name rke2-tf-test-${IMGNAME} -t \
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
	@docker logs -f rke2-tf-test-${IMGNAME}

.PHONY: tf-tests-down
tf-tests-down:
	@echo "Removing containers"
	@docker stop $$(docker ps -a -q --filter="name=rke2-tf*")
	@docker rm $$(docker ps -a -q --filter="name=rke2-tf*")



tf-tests-clean:
	@./terraform/scripts/delete_resources.sh

.PHONY: tf-tests
tf-tests: tf-tests-clean tf-tests-down tf-tests-up tf-tests-run


#========================= Run terraform tests locally =========================#
.PHONY: tf-tests-local-createcluster
tf-tests-local-createcluster:
	@go test -timeout=40m -v ./terraform/createcluster/...

.PHONY: tf-tests-local-upgradecluster
tf-tests-local-upgradecluster:
	@go test -timeout=45m -v ./terraform/upgradecluster/... -${ARGVALUE}=${ARGNAME}


#========================= TestCode Static Quality Check =========================#
.PHONY: vet-lint                   ## Run locally only inside Tests package
vet-lint:
	@echo "Running go vet and lint"
	@go vet ./${TESTDIR}
	@cd ${TESTDIR} && golangci-lint run --tests