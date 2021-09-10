# Testing Standards in RKE2

Go testing in RKE2 comes in 2 forms: Unit, Integration. This document will 
explain *when* each test should be written and *how* each test should be
generated, formatted, and run.

Note: all shell commands given are relateive to the root RKE2 repo directory.
___

## Unit Tests

Unit tests should be written when a component or function of a package needs testing.
Unit tests should be used for "white box" testing.

### Framework

All unit tests in RKE2 follow a [Table Driven Test](https://github.com/golang/go/wiki/TableDrivenTests) style. Specifically, RKE2 unit tests are automatically generated using the [gotests](https://github.com/cweill/gotests) tool. This is built into the Go vscode extension, has documented integrations for other popular editors, or can be run via command line. Additionally, a set of custom templates are provided to extend the generated test's functionality. To use these templates, call:

```bash
gotests --template_dir=<PATH_TO_RKE2>/contrib/gotests_templates
```

Or in vscode, edit the Go extension setting `Go: Generate Tests Flags`  
and add `--template_dir=<PATH_TO_RKE2>/contrib/gotests_templates` as an item.

### Format

All unit tests should be placed within the package of the file they test.  
All unit test files should be named: `<FILE_UNDER_TEST>_test.go`.  
All unit test functions should be named: `Test_Unit<FUNCTION_TO_TEST>` or `Test_Unit<RECEIVER>_<METHOD_TO_TEST>`.  
See the [service account unit test](https://github.com/rancher/rke2/blob/master/pkg/rke2/serviceaccount_test.go) as an example.

### Running

```bash
go test ./pkg/... -run Unit
```

Note: As unit tests call functions directly, they are the primary drivers of RKE2's code coverage
metric.

___

## Integration Tests

Integration tests should be used to test a specific functionality of RKE2 that exists across multiple Go packages, either via exported function calls, or more often, CLI comands.
Integration tests should be used for "black box" testing. 

### Framework

All integration tests in RKE2 follow a [Behavior Diven Development (BDD)](https://en.wikipedia.org/wiki/Behavior-driven_development) style. Specifically, RKE2 uses [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) to drive the tests.  
To generate an initial test, the command `ginkgo bootstrap` can be used.

To facilitate RKE2 CLI testing, see `tests/util/cmd.go` helper functions.

### Format

All integration tests should be places in `tests` directory.
All integration test files should be named: `<TEST_NAME>_int_test.go`  
All integration test functions should be named: `Test_Integration<Test_Name>`.  
See the [etcd snapshot test](https://github.com/rancher/rke2/blob/master/tests/etcd_int_test.go) as an example.  

### Running

Integration tests must be with an existing single-node cluster, tests will skip if the server is not configured correctly.
```bash
make dev-shell
# Once in the dev-shell
# Start rke2 server with appropriate flags
./bin/rke2 server 
```
Open another terminal
```bash
make dev-shell-enter
# once in the dev-shell
go test ./tests/ -run Integration
```

## Contributing New Or Updated Tests

___
If you wish to create a new test or update an existing test, 
please submit a PR with a title that includes the words `<NAME_OF_TEST> (Created/Updated)`.
