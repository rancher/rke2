## Acceptance Test Framework

The acceptance tests are a customizable way to create clusters and perform validations on them such that the requirements of specific features and functions can be validated.

- It relies on [Terraform](https://www.terraform.io/) to provide the underlying cluster configuration.
- It uses [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) as assertion framework.


## Architecture
- For better maintenance, readability and productivity we encourage max of separation of concerns and loose coupling between packages so inner packages should not depend on outer packages

### Packages:
```bash
./acceptance
│
├── core
│   └───── Place where resides the logic and services for it
|
├── entrypoint
│   └───── Where is located the entrypoint for tests execution, separated by test runs and test suites
│ 
├── fixtures
│   └───── Place where resides the fixtures for tests
│
├── modules
│   └───── Terraform modules and configurations
│
├── shared
│   └───── shared and reusable functions, workloads, constants, and scripts

```

 

### Explanation:

- `Core`
```
    Service:
    
Act:                  Acts as a provider for customizations across framework
Responsibility:       Should not depend on any outer layer only in the core itself, the idea is to provide not rely on.
     
     
    Testcase:   

Act:                  Acts as a innermost layer where the main logic (test implementations) is handled.
Responsibility:       Encapsulate test logic and should not depend on any outer layer

```

- `Entrypoint`
````
Act:                  Acts as the one of the outer layer to receive the input to start test execution
Responsibility:       Should not need to implement any logic and only focus on orchestrating
````

- `Fixtures`
```
Act:                  It acts as a provider for test fixtures
Responsibility:       Totally independent of any other layer and should only provide
```

- `Modules`
```
Act:                  It acts as the infra to provide the terraform modules and configurations
Responsibility:       Only provides indirectly for all, should not need the knowledge of any test logic or have dependencies from internal layers.
``` 

- `Shared`
```
Act:                  It acts as an intermediate module providing shared and reusable functions, constants, and scripts               
Responsibility:       Should not need the knowledge or "external" dependency at all, provides for all.
```

#### PS: "External" and "Outer" dependency here in this context is considered any other package within the acceptance framework.

-------------------

### `Template Bump Version Model `

- We have a template model interface for testing bump versions, the idea is to provide a simple and direct way to test bump of version using go test tool.


```You can test that like:```

- Adding one version or commit and ran some commands on it and check it against respective expected values then upgrade and repeat the same commands and check the respective new (or not) expected values.


```How can I do that?```

- Step 1: Add your desired first version or commit that you want to use on `local.tfvars` file on the vars `rke2_version` and `install_mode`
- Step 2: Have the commands you need to run and the expected output from them
- Step 3: Have a version or commit that you want to upgrade to.
- Step 4: Create your go test file in `acceptance/entrypoint/versionbump/versionbump{mytestname}.go`.
- Step 5: Get the template from `acceptance/entrypoint/versionbump/versionbump.go` and copy it to your test file.
- Step 6: Fill the template with your data ( RunCmdNode and RunCmdHost) with your respective commands.
- Step 7: On the TestConfig field you can add another test case that we already have or a newly created one.
- Step 8: Create the go test command and the make command to run it.
- Step 9: Run the command and wait for results.
- Step 10: (WIP) Export your customizable report.


-------------------
- RunCmdNode:
  Commands like:
    - $ `curl ...`
    - $ `sudo chmod ...`
    - $ `sudo systemctl ...`
    - $ `grep ...`
    - $ `rke2 --version`


- RunCmdHost:
  Basically commands like:
    - $ `kubectl ...`
    - $ `helm ...`



Available arguments to create your command with examples:
````
- $ -cmdHost kubectl describe pod -n kube-system local-path-provisioner-,  | grep -i Image"
- $ -expectedValueHost "v0.0.21"
- $ -expectedValueUpgradedHost "v0.0.24"
- $ -cmdNode "rke2 --version"
- $ -expectedValueNode "v1.25.2+k3s1"
- $ -expectedValuesUpgradedNode "v1.26.4-rc1+rke2r1"
- $ -upgradeVersionSUC "v1.26.4-rc1+rke2r1"
- $ -installtype INSTALL_RKE2_COMMIT=257fa2c54cda332e42b8aae248c152f4d1898218
- $ -deployWorkload true
- $ -testCase TestCaseName
- $ -description "Description of your test"
````

Example of an execution considering that the `commands` are already placed or inside your test function or the *template itself (example below the command):
```bash
 go test -v -timeout=45m -tags=coredns ./entrypoint/versionbump/... \                     
  -expectedValueHost "v1.9.3"  \       
  -expectedValueUpgradedHost "v1.10.1" \
  -expectedValueNode "v1.25.9+rke2r1" \            
  -expectedValueUpgradedNode "v1.26.4-rc1+rke2r1" \                                  
  -installType INSTALL_RKE2_VERSION=v1.25.9+rke2r1
  
````
PS: If you need to send more than one command at once split them with  " , "

#### `*template with commands and testcase added:`
```go
- Values passed in code.
-------------------------------------------------------
- util.GetCoreDNSdeployImage
- testcase.TestCoredns
-------------------------------------------------------
	
	
Value passed in command line 
-------------------------------------------------------
-expectedValueHost "v1.9.3" 
-expectedValueUpgradedHost "v1.10.1" 
-------------------------------------------------------

	
    template.VersionTemplate(template.VersionTestTemplate{
	    Description: "Test CoreDNS bump",
        TestCombination: &template.RunCmd{
                RunOnHost: []template.TestMap{
                {
                    Cmd:                  util.GetCoreDNSdeployImage,
                    ExpectedValue:        service.ExpectedValueHost,
                    ExpectedValueUpgrade: service.ExpectedValueUpgradedHost,
                },
                },
		},
        InstallUpgrade: customflag.installType,
        TestConfig: &template.TestConfig{
            TestFunc:       testcase.TestCoredns,
            DeployWorkload: true,
        },
	})
})

````


#### You can also run a totally parametrized test with the template, just copy and paste the template and call everything as flags like that:
- Template
```` go
	template.VersionTemplate(GinkgoT(), template.VersionTestTemplate{
			Description: util.Description,
			TestCombination: &template.RunCmd{
				RunOnNode: []template.TestMap{
					{
						Cmd:                  util.CmdNode,
						ExpectedValue:        util.ExpectedValueNode,
						ExpectedValueUpgrade: util.ExpectedValueUpgradedNode,
					},
				},
				RunOnHost: []template.TestMap{
					{
						Cmd:                  util.CmdHost,
						ExpectedValue:        util.ExpectedValueHost,
						ExpectedValueUpgrade: util.ExpectedValueUpgradedHost,
					},
				},
			},
			InstallUpgrade: util.installType,
			TestConfig: &template.TestConfig{
				TestFunc:       template.TestCase(util.TestCase.TestFunc),
				DeployWorkload: util.TestCase.DeployWorkload,
			},
		})
	})
````

- Command
```bash
go test -timeout=45m -v -tags=versionbump ./entrypoint/versionbump/...  \   
  -cmdNode "rke2 --version" \
  -expectedValueNode "v1.25.3+rke2r1"  \
  -expectedValueUpgradedNode "v1.25.9+rke2r1" \
  -cmdHost "kubectl get deploy rke2-coredns-rke2-coredns -n kube-system -o jsonpath='{.spec.template.spec.containers[?(@.name==\"coredns\")].image}'"  \
  -expectedValueHost "v1.9.3" \
  -expectedValueUpgradedHost "v1.10.1" \
  -installtype INSTALL_RKE2_VERSION=v1.25.9+rke2r1 \
  -testCase "TestCoredns" \
  -deployWorkload true
                            
````

#### We also have this on the `makefile` to make things easier to run just adding the values, please see below on the makefile section


-----
#### Testcase naming convention:
- All tests should be placed under `tests/acceptance/testcase/<TESTNAME>`.
- All test functions should be named: `Test<TESTNAME>`.



## Running

- Before running the tests, you should creat local.tfvars file in `./tests/acceptance/modules/config/local.tfvars`. There is some information there to get you started, but the empty variables should be filled in appropriately per your AWS environment.

- Please make sure to export your correct AWS credentials before running the tests. e.g:  
```bash
export AWS_ACCESS_KEY_ID=<YOUR_AWS_ACCESS_KEY_ID>
export AWS_SECRET_ACCESS_KEY=<YOUR_AWS_SECRET_ACCESS_KEY>
```

- The local.tfvars split roles section should be strictly followed to not cause any false positives or negatives on tests

- Please also when creating tf var resource_name, make sure that you do not have any instances from other automations with the same name to avoid deleting wrong resources


*** 

Tests can be run individually per package or per test tags from acceptance package:
```bash
go test -timeout=45m -v ./entrypoint/$PACKAGE_NAME/...

go test -timeout=45m -v ./entrypoint/upgradecluster/... -installtype INSTALL_RKE2_VERSION=v1.25.8+rke2r1 -upgradeVersion v1.25.8+rke2r1

go test -timeout=45m -v -tags=upgrademanual ./entrypoint/upgradecluster/... -installtype INSTALL_RKE2_VERSION=v1.25.8+rke2r1

go test -timeout=45m -v -tags=upgradesuc ./entrypoint/upgradecluster/... -upgradeVersionSUC v1.25.8+rke2r1

```
Test flags:
```
 ${upgradeVersion} version to upgrade to
    -upgradeVersionSUC v1.26.2+rke2r1
    
 ${installType} type of installation (version or commit) + desired value    
    -installType Version or commit
```
Test tags:
```
 -tags=upgradesuc
 -tags=upgrademanual
 -tags=versionbump
 -tags=runc
 -tags=coredns
```
###  Run with `Makefile` through acceptance package:
```bash
- On the first run with make and docker please delete your .terraform folder, terraform.tfstate and terraform.hcl.lock file

Args:
*Most of args are optional so you can fit to your use case.

- ${IMGNAME}               append any string to the end of image name
- ${TAGNAME}               append any string to the end of tag name
- ${ARGNAME}               name of the arg to pass to the test
- ${ARGVALUE}              value of the arg to pass to the test
- ${TESTDIR}               path to the test directory 
- ${TESTFILE}              path to the test file
- ${TAGTEST}               name of the tag function from suite ( -tags=upgradesuc or -tags=upgrademanual )
- ${TESTCASE}              name of the testcase to run
- ${DEPLOYWORKLOAD}        true or false to deploy workload
- ${CMDHOST}               command to run on host
- ${VALUEHOST}             value to check on host
- ${VALUEHOSTUPGRADED}     value to check on host after upgrade
- ${CMDNODE}               command to run on node
- ${VALUENODE}             value to check on node
- ${VALUENODEUPGRADED}     value to check on node after upgrade
- ${INSTALLTYPE}           type of installation (version or commit) + desired value



Commands: 
$ make test-env-up                     # create the image from Dockerfile.build
$ make test-run                        # runs create and upgrade cluster by passing the argname and argvalue
$ make test-env-down                   # removes the image and container by prefix
$ make test-env-clean                  # removes instances and resources created by testcase
$ make test-logs                       # prints logs from container the testcase
$ make test-create                     # runs create cluster test locally
$ make test-version-runc               # runs version runc test locally
$ make test-version-coredns            # runs version coredns test locally
$ make test-upgrade-suc                # runs upgrade via SUC
$ make test-upgrade-manual             # runs upgrade manually
$ make test-version-bump               # runs version bump test locally
$ make test-run                        # runs create and upgrade cluster by passing the argname and argvalue
$ make remove-tf-state                 # removes acceptance state dir and files
$ make test-suite                      # runs all testcase locally in sequence not using the same state
$ make vet-lint                        # runs go vet and go lint
```

### Examples with docker:
```
- Create an image tagged
$ make test-env-up TAGNAME=ubuntu

- Run upgrade cluster test with `${IMGNAME}` and  `${TAGNAME}`
$ make test-run IMGNAME=2 TAGNAME=ubuntu


- Run create and upgrade cluster just adding `INSTALLTYPE` flag to upgrade
$ make test-run


- Run version bump test upgrading with commit id
$ make test-run IMGNAME=x \
  TAGNAME=y \
  TESTDIR=versionbump \
  CMDNODE="rke2 --version" \
  VALUENODE="v1.26.2+rke2r1" \
  CMDHOST="kubectl get image..."  \
  VALUEHOST="v0.0.21" \
  INSTALLTYPE=INSTALL_RKE2_COMMIT=257fa2c54cda332e42b8aae248c152f4d1898218 \ 
  TESTCASE=TestCaseName \
  DEPLOYWORKLOAD=true
````

### Examples to run locally:
````
- Run create cluster test:
$ make test-create

- Run upgrade cluster test:
$ make test-upgrade-manual INSTALLTYPE=v1.26.2+k3s1


- Run bump version for coreDNS test
$ make test-version-bump \
   CMDNODE='rke2 --version' \
   VALUENODE="v1.25.3+rke2r1" \
   VALUENODEUPGRADED="v1.25.9+rke2r1" \
   CMDHOST='kubectl get deploy rke2-coredns-rke2-coredns -n kube-system -o json, | grep "rancher/hardened-coredns"  \
   VALUEHOST="v1.9.3" \
   VALUEHOSTUPGRADED="v1.10.1" \
   INSTALLTYPE=INSTALL_RKE2_VERSION=v1.25.9+rke2r1 \
   TESTCASE="TestCoredns" \
   DEPLOYWORKLOAD=true
    
    
    
- Run bump version for runc test
$ make test-version-runc \
   CMDHOST='kubectl get nodes' \
   VALUEHOST="Ready" \
   VALUEHOSTUPGRADED="Ready" \
   CMDNODE="(find /var/lib/rancher/rke2/data/ -type f -name runc -exec {} --version \\;)"  \
   VALUENODE="1.1.4" \
   VALUENODEUPGRADED="1.1.5" \
   INSTALLTYPE=INSTALL_RKE2_VERSION=v1.25.9+rke2r1



- Logs from test
$ make tf-logs IMGNAME=1

- Run lint for a specific directory
$ make vet-lint TESTDIR=upgradecluster

````


### Running tests in parallel:

- You can play around and have a lot of different test combinations like:
```
- Build docker image with different TAGNAME="OS`s" + with different configurations( resource_name, node_os, versions, install type, nodes and etc) and have unique "IMGNAMES"

- And in the meanwhile run also locally with different configuration while your dockers TAGNAME and IMGNAMES are running
```


### In between tests:
````
- If you want to run with same cluster do not delete ./tests/acceptance/modules/terraform.tfstate + .terraform.lock.hcl file after each test.

- If you want to use new resources then make sure to delete the ./tests/acceptance/modules/terraform.tfstate + .terraform.lock.hcl file if you want to create a new cluster.
````

### Common Issues:
```
- Issues related to terraform plugin please also delete the modules/.terraform folder
- Issues related to terraform failed to find local token , please also delete modules/.terraform folder
- In mac m1 maybe you need also to go to rke2/tests/terraform/modules and run `terraform init` to download the plugins
```

### Debugging
````
To focus individual runs on specific test clauses, you can prefix with `F`. For example, in the [create cluster test](../tests/acceptance/entrypoint/createcluster_test.go), you can update the initial creation to be: `FIt("Starts up with no issues", func() {` in order to focus the run on only that clause.
Or use break points in your IDE.
````
