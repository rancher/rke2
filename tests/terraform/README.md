# Terraform (TF) Tests

Terraform (TF) tests are an additional form of End-to-End (E2E) tests that cover multi-node RKE2 configuration and administration: install, update, teardown, etc. across a wide range of operating systems. Terraform tests are used as part of RKE2 quality assurance (QA) to bring up clusters with different configurations on demand, perform specific functionality tests, and keep them up and running to perform some exploratory tests in real-world scenarios.

## Framework 
TF tests utilize [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) like the e2e tests. They rely on [Terraform](https://www.terraform.io/) to provide the underlying cluster configuration. 

## Format

- All TF tests should be placed under `tests/terraform/<TEST_NAME>`.
- All TF test functions should be named: `Test_TF<TEST_NAME>`. 

See the [create cluster test](../tests/terraform/createcluster_test.go) as an example.

## Running

Before running the tests, it's required to create a tfvars file in `./tests/terraform/modules/config/local.tfvars`. This should be filled in to match the desired variables, including those relevant for your AWS environment. All variables that are necessary can be seen in [main.tf](../tests/terraform/modules/main.tf).
It is also required to have standard AWS environment variables present: `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`

All TF tests can be run with:
```bash
go test -timeout=60m -v ./tests/terrfaorm/... -run TF
```
Tests can be run individually with:
```bash
go test -timeout=30m -v ./tests/terraform/createcluster_test.go ./tests/terraform/cluster.go ./tests/terraform/testutils.go
# OR
go test -timeout=30m -v ./tests/terraform/... -run TFClusterCreateValidation
```
Additionally, you can use docker to run the tests, which may be beneficial when wanting to run multiple tests in parallel. Just be sure to change the resource name in the tfvars file to ensure there won't be overwrites!
```bash
$ docker build . -f ./tests/terraform/scripts/Dockerfile.build -t rke2-tf
# This next command assumes you have the following environment variable in your config/local.tfvars: 'access_key = "/tmp/aws_key.pem"'
$ docker run --name rke2-tf-test1 -t -e AWS_ACCESS_KEY_ID=<YOUR_ACCESS_KEY> -e AWS_SECRET_ACCESS_KEY=<YOUR_SECRET_KEY> -v /path/to/aws/key.pem:/tmp/aws_key.pem rke2-tf sh -c "go test -timeout=30m -v ./tests/terraform/createcluster_test.go ./tests/terraform/cluster.go ./tests/terraform/testutils.go"
```

In between tests, if the cluster is not destroyed, then make sure to delete the ./tests/terraform/modules/terraform.tfstate file if you want to create a new cluster.


# Debugging
To focus individual runs on specific test clauses, you can prefix with `F`. For example, in the [create cluster test](../tests/terraform/createcluster_test.go), you can update the initial creation to be: `FIt("Starts up with no issues", func() {` in order to focus the run on only that clause.
