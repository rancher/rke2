##=========================  Acceptance Tests =========================#
include ./config.mk

TAGNAME ?= default
test-env-up:
	@cd ../.. && docker build . -q -f ./tests/acceptance/shared/scripts/Dockerfile.build -t rke2-automated-${TAGNAME}

# -d
.PHONY: test-run
test-run:
	docker run  --name rke2-automated-test-${IMGNAME} -t \
      -e AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" \
      -e AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" \
      -v ${ACCESS_KEY_LOCAL}:/go/src/github.com/rancher/rke2/tests/acceptance/modules/config/.ssh/aws_key.pem \
       rke2-automated-${TAGNAME} sh -c 'cd ./tests/acceptance/entrypoint; \
                                if [ -n "${TESTDIR}" ]; then \
                                    if [ "${TESTDIR}" = "upgradecluster" ]; then \
                                        if [ "${TESTTAG}" = "upgradesuc" ]; then \
                                            go test -timeout=45m -v -tags=upgradesuc ./upgradecluster/... -upgradeVersionSUC "${ARGVALUE}"; \
                                        elif [ "${TESTTAG}" = "upgrademanual" ]; then \
                                            go test -timeout=45m -v -tags=upgrademanual ./upgradecluster/... -installtype "${INSTALLTYPE}"; \
                                        fi; \
                                    elif [ "${TESTDIR}" = "versionbump" ]; then \
                                        if [ "${TESTTAG}"  =  "versionbump" ]; then \
                                            go test -timeout=45m -v -tags=versionbump ./versionbump/... -cmdNode "${CMDNODE}" -expectedValueNode "${VALUENODE}" \
                                            -expectedValueUpgradedNode "${VALUENODEUPGRADED}" -cmdHost "${CMDHOST}" -expectedValueHost "${VALUEHOST}" \
                                            -expectedValueUpgradedHost "${VALUEHOSTUPGRADED}" -installUpgradeFlag "${INSTALLTYPE}" -testCase "${TESTCASE}" \
                                            -deployWorkload "${DEPLOYWORKLOAD}"; \
                                            -description "${DESCRIPTION}"; \
                                        elif [ "${TESTTAG}"  = "coredns" ]; then \
                                            go test -timeout=45m -v -tags=coredns ./versionbump/... -expectedValueNode "${VALUENODE}" \
                                            -expectedValueUpgradedNode "${VALUENODEUPGRADED}" -expectedValueHost "${VALUEHOST}" \
                                            -expectedValueUpgradedHost "${VALUEHOSTUPGRADED}" -installUpgradeFlag "${INSTALLTYPE}"; \
                                        elif [ "${TESTTAG}"  = "runc" ]; then \
                                            go test -timeout=45m -v -tags=runc ./versionbump/... \
                                            -expectedValueNode ${VALUENODE} \
                                            -expectedValueUpgradedNode ${VALUENODEUPGRADED} \
                                            -installUpgradeFlag ${INSTALLTYPE}; \
                                        fi; \
                                    fi; \
                                elif [ -z "${TESTDIR}" ]; then \
                                    go test -timeout=45m -v ./createcluster/...; \
                                fi;'


.PHONY: test-logs
test-logs:
	@docker logs -f rke2-automated-test-${IMGNAME}


.PHONY: test-env-down
test-env-down:
	@echo "Removing containers and images"
	@docker stop $$(docker ps -a -q --filter="name=rke2-automated*")
	@docker rm $$(docker ps -a -q --filter="name=rke2-automated*")
	@docker rmi $$(docker images -q --filter="reference=rke2-automated*")


.PHONY: test-env-clean
test-env-clean:
	@./shared/scripts/delete_resources.sh


.PHONY: test-complete
test-complete: test-env-clean test-env-down remove-tf-state test-env-up test-run


.PHONY: remove-tf-state
remove-tf-state:
	@rm -rf ./modules/.terraform
	@rm -rf ./modules/.terraform.lock.hcl ./modules/terraform.tfstate ./modules/terraform.tfstate.backup


#======================== Run acceptance tests locally =========================#

.PHONY: test-create
test-create:
	@go test -timeout=45m -v ./entrypoint/createcluster/...


.PHONY: test-upgrade-suc
test-upgrade-suc:
	@go test -timeout=45m -v -tags=upgradesuc  ./entrypoint/upgradecluster/... -upgradeVersionSUC ${ARGVALUE}


.PHONY: test-upgrade-manual
test-upgrade-manual:
	@go test -timeout=45m -v -tags=upgrademanual ./entrypoint/upgradecluster/... -installtype ${INSTALLTYPE}


.PHONY: test-version-runc
test-version-runc:
	@go test -timeout=45m -v -tags=runc ./entrypoint/versionbump/... \
		-expectedValueNode ${VALUENODE} \
		-expectedValueUpgradedNode ${VALUENODEUPGRADED} \
		-installUpgradeFlag ${INSTALLTYPE} \



.PHONY: test-version-coredns
test-version-coredns:
	@go test -timeout=45m -v -tags=coredns ./entrypoint/versionbump/... \
		-expectedValueHost ${VALUEHOST} \
		-expectedValueUpgradedHost ${VALUEHOSTUPGRADED} \
		-installUpgradeFlag ${INSTALLTYPE} \


.PHONY: test-version-bump
test-version-bump:
	@go test -timeout=45m -v -tags=versionbump ./entrypoint/versionbump/... \
		-cmdNode "${CMDNODE}" \
		-expectedValueNode ${VALUENODE} \
		-expectedValueUpgradedNode ${VALUENODEUPGRADED} \
		-cmdHost "${CMDHOST}" \
		-expectedValueHost ${VALUEHOST} \
		-expectedValueUpgradedHost ${VALUEHOSTUPGRADED} \
		-installUpgradeFlag ${INSTALLTYPE} \
		-testCase ${TESTCASE} \
		-deployWorkload ${DEPLOYWORKLOAD}


#========================= TestCode Static Quality Check =========================#
.PHONY: vet-lint                      ## Run locally only inside Tests package
vet-lint:
	@echo "Running go vet and lint"
	@go vet ./${TESTDIR} && golangci-lint run --tests