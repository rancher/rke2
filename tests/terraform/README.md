# Terraform (TF) Tests

Terraform (TF) tests are an additional form of End-to-End (E2E) tests that cover multi-node RKE2 configuration and administration: install, update, teardown, etc. across a wide range of operating systems. Terraform tests are used as part of RKE2 quality assurance (QA) to bring up clusters with different configurations on demand, perform specific functionality tests, and keep them up and running to perform some exploratory tests in real-world scenarios.

## Framework 
TF tests utilize [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) like the e2e tests. They rely on [Terraform](https://www.terraform.io/) to provide the underlying cluster configuration. 

## Format

- The Terraform package architecture is as follows:
```bash
./terraform
├── cases    -------------------> Contains the tests cases and suite separated
├── core     -------------------> Contains the base core test helpers
│── modules  -------------------> Contains the terraform modules and configurations
│── shared   -------------------> Contains the shared functions, constants and scripts
|── testfunctions  -------------> Contains the tests implementations itself
```

- All TF tests should be placed under `tests/terraform/cases/<TEST_NAME>`.
- All TF test functions should be named: `TestTF<TEST_NAME>`. 


## Running

- Before running the tests, it's required to create a tfvars file in `./tests/terraform/modules/config/local.tfvars`. This should be filled in to match the desired variables, including those relevant for your AWS environment. All variables that are necessary can be seen in [main.tf](../tests/terraform/modules/main.tf).
It is also required to have standard AWS environment variables present: `AWS_ACCESS_KEY_ID` , `AWS_SECRET_ACCESS_KEY` and `ACCESS_KEY_LOCAL`


- The local.tfvars split roles section should be strictly followed to not cause any false positives or negatives on tests


- Please also when creating tf var resource_name, make sure that you do not have any instances from other automations with the same name to avoid deleting wrong resources


- If you want to run tests locally totally in parallel, please make sure that you have different resource_name for each test

*** 

Tests can be run per package with:
```bash
go test -timeout=30m -v ./tests/terraform/cases/$PACKAGE_NAME/...
```

go test flags:
```
- ${upgradeVersion} version to upgrade to
```
###  Run with `Makefile` through terraform package:


```
- On the first run with make and docker please delete your .terraform folder, terraform.tfstate and terraform.hcl.lock file


Args:
*All args are optional and can be used with:

`$make tf-run`         `$make tf-logs`,
`$make vet-lint`       `$make tf-complete`, 
`$make tf-upgrade`     `$make tf-test-suite-same-cluster`,
`$make tf-test-suite`

- ${IMGNAME}     append any string to the end of image name
- ${TAGNAME}     append any string to the end of tag name
- ${ARGNAME}     name of the arg to pass to the test
- ${ARGVALUE}    value of the arg to pass to the test
- ${TESTDIR}     path to the test directory 

Commands:
$ make tf-up                         # create the image from Dockerfile.build
$ make tf-run                        # runs all tests if no flags or args provided
$ make tf-down                       # removes the image
$ make tf-clean                      # removes instances and resources created by tests
$ make tf-logs                       # prints logs from container the tests
$ make tf--complete                  # clean resources + remove images + run tests
$ make tf-create                     # runs create cluster test locally
$ make tf-upgrade                    # runs upgrade cluster test locally
$ make tf-test-suite-same-cluster    # runs all tests locally in sequence using the same state    
$ make tf-remove-state               # removes terraform state dir and files
$ make tf-test-suite                 # runs all tests locally in sequence not using the same state
$ make vet-lint                      # runs go vet and go lint

     
Examples:
$ make tf-up TAGNAME=ubuntu
$ make tf-run IMGNAME=2 TAGNAME=ubuntu TESTDIR=upgradecluster ARGNAME=upgradeVersion ARGVALUE=v1.26.2+rke2r1
$ make tf-run TESTDIR=createcluster
$ make tf-logs IMGNAME=1
$ make vet-lint TESTDIR=upgradecluster


```
 ###  Running tests in parallel:
```
- You can play around and have a lot of different test combinations like:
- Build docker image with different TAGNAME="OS`s" + with different configurations( resource_name, node_os, versions, install type, nodes and etc) and have unique "IMGNAMES"
- And in the meanwhile run also locally with different configuration while your dockers TAGNAME and IMGNAMES are running
```

### In between tests:
````
- If you want to run with same cluster do not delete ./tests/terraform/modules/terraform.tfstate + .terraform.lock.hcl file after each test.

- If you want to use new resources then make sure to delete the ./tests/terraform/modules/terraform.tfstate + .terraform.lock.hcl file if you want to create a new cluster.
````

### Common Issues:
```
- Issues related to terraform plugin please also delete the modules/.terraform folder
- Issues related to terraform failed to find local token , please also delete modules/.terraform folder
- In mac m1 maybe you need also to go to rke2/tests/terraform/modules and run `terraform init` to download the plugins

```

### Debugging
````
To focus individual runs on specific test clauses, you can prefix with `F`. For example, in the [create cluster test](../tests/terraform/cases/createcluster_test.go), you can update the initial creation to be: `FIt("Starts up with no issues", func() {` in order to focus the run on only that clause.
