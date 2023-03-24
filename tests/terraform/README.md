# Terraform (TF) Tests

Terraform (TF) tests are an additional form of End-to-End (E2E) tests that cover multi-node RKE2 configuration and administration: install, update, teardown, etc. across a wide range of operating systems. Terraform tests are used as part of RKE2 quality assurance (QA) to bring up clusters with different configurations on demand, perform specific functionality tests, and keep them up and running to perform some exploratory tests in real-world scenarios.

## Framework 
TF tests utilize [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) like the e2e tests. They rely on [Terraform](https://www.terraform.io/) to provide the underlying cluster configuration. 

## Format

- All TF tests should be placed under `tests/terraform/<TEST_NAME>`.
- All TF test functions should be named: `Test_TF<TEST_NAME>`. 

See the [create cluster test](../tests/terraform/createcluster_test.go) as an example.

## Running

- Before running the tests, it's required to create a tfvars file in `./tests/terraform/modules/config/local.tfvars`. This should be filled in to match the desired variables, including those relevant for your AWS environment. All variables that are necessary can be seen in [main.tf](../tests/terraform/modules/main.tf).
It is also required to have standard AWS environment variables present: `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`


- The local.tfvars split roles section should be strictly followed to not cause any false positives or negatives on tests


- Please also when creating tf var resource_name, make sure that you do not have any instances from other automations with the same name to avoid deleting wrong resources


- If you want to run tests locally totally in parallel, please make sure that you have different resource_name for each test

*** 

Tests can be run per package with:
```bash
go test -timeout=30m -v ./tests/terraform/$PACKAGE_NAME/...
```
Additionally, you can use docker to run the tests, which may be beneficial when wanting to run multiple tests in parallel. Just be sure to change the resource name in the tfvars file to ensure there won't be overwrites! Provided example below is for running two separate packages using docker:
```bash
$ docker build . -f ./tests/terraform/scripts/Dockerfile.build -t rke2-tf
# These next commands assume you have the following environment variable in your config/local.tfvars: 'access_key = "/tmp/aws_key.pem"'
$ docker run --name rke2-tf-creation-test -t -e AWS_ACCESS_KEY_ID=<YOUR_ACCESS_KEY> -e AWS_SECRET_ACCESS_KEY=<YOUR_SECRET_KEY> -v /path/to/aws/key.pem:/tmp/aws_key.pem rke2-tf sh -c "go test -timeout=30m -v ./tests/terraform/createcluster/..."
$ docker run --name rke2-tf-upgrade-test -t -e AWS_ACCESS_KEY_ID=<YOUR_ACCESS_KEY> -e AWS_SECRET_ACCESS_KEY=<YOUR_SECRET_KEY> -v /path/to/aws/key.pem:/tmp/aws_key.pem rke2-tf sh -c "go test -timeout=45m -v ./tests/terraform/upgradecluster/... -upgradeVersion=v1.24.8+rke2r1"
```
Test Flags:
```
- ${upgradeVersion} version to upgrade to
```
We can also run tests through the Makefile through tests' directory:
```bash
Args:
*All args are optional and can be used with `$make tf-tests-run` , `$make tf-tests-logs` and `$make vet-lint`

- ${IMGNAME}     append any string to the end of image name
- ${ARGNAME}     name of the arg to pass to the test
- ${ARGVALUE}    value of the arg to pass to the test
- ${TESTDIR}     path to the test directory 

Commands:
$ make tdf-tests-up                  # create the image from Dockerfile.build
$ make tf-tests-run                  # runs all tests if no flags or args provided
$ make tf-tests-down                 # removes the image
$ make tf-tests-clean                # removes instances and resources created by tests
$ make tf-tests-logs                 # prints logs from container the tests
$ make tf-tests                      # clean resources + remove images + run tests
$ make vet-lint                      # runs go vet and go lint
$ make tf-tests-local-createcluster  # runs create cluster test locally
$ make tf-tests-local-upgradecluster # runs upgrade cluster test locally

Examples:
$ make tf-tests-run IMGNAME=2 TESTDIR=terraform/upgradecluster ARGNAME=upgradeVersion ARGVALUE=v1.26.2+rke2r1
$ make tf-tests-run TESTDIR=terraform/upgradecluster
$ make tf-tests-logs IMGNAME=1
$ make vet-lint TESTDIR=terraform/upgradecluster
```


In between tests, if the cluster is not destroyed, then make sure to delete the ./tests/terraform/modules/terraform.tfstate + .terraform.lock.hcl file if you want to create a new cluster.


# Common Issues:
````
- Issues related to terraform plugin please also delete the /.terraform folder and or go to folder that have main.tf and just run `terraform init` to download the plugin again
````

# Debugging
To focus individual runs on specific test clauses, you can prefix with `F`. For example, in the [create cluster test](../tests/terraform/createcluster_test.go), you can update the initial creation to be: `FIt("Starts up with no issues", func() {` in order to focus the run on only that clause.
