package versionbump

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/rancher/rke2/tests/acceptance/core/service/customflag"
	"github.com/rancher/rke2/tests/acceptance/core/service/factory"
	"github.com/rancher/rke2/tests/acceptance/core/service/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMain(m *testing.M) {
	flag.StringVar(&template.TestMapFlag.Cmd, "cmd", "", "Comma separated list of commands to execute")
	flag.StringVar(&template.TestMapFlag.ExpectedValue, "expectedValue", "", "Comma separated list of expected values for commands")
	flag.StringVar(&template.TestMapFlag.ExpectedValueUpgrade, "expectedValueUpgrade", "", "Expected value of the command ran on Node after upgrading")
	flag.Var(&customflag.ServiceFlag.InstallUpgrade, "installVersionOrCommit", "Install upgrade customflag")
	flag.StringVar(&customflag.ServiceFlag.InstallType.Channel, "channel", "", "channel to use on install or upgrade")
	flag.Var(&customflag.TestCaseNameFlag, "testCase", "Comma separated list of test case names to run")
	flag.BoolVar(&customflag.ServiceFlag.TestConfig.DeployWorkload, "deployWorkload", false, "Deploy workload customflag")
	flag.StringVar(&customflag.ServiceFlag.TestConfig.WorkloadName, "workloadName", "", "Name of the workload to a standalone deploy")
	flag.Var(&customflag.ServiceFlag.ClusterConfig.Destroy, "destroy", "Destroy cluster after test")
	flag.StringVar(&customflag.ServiceFlag.TestConfig.Description, "description", "", "Description of the test")
	flag.Parse()

	installVersionOrCommit := strings.Split(customflag.ServiceFlag.InstallUpgrade.String(), ",")
	customflag.ServiceFlag.InstallUpgrade = installVersionOrCommit

	customflag.ServiceFlag.TestConfig.TestFuncNames = customflag.TestCaseNameFlag
	testFuncs, err := template.AddTestCases(customflag.ServiceFlag.TestConfig.TestFuncNames)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	if len(testFuncs) > 0 {
		testCaseFlags := make([]customflag.TestCaseFlag, len(testFuncs))
		for i, j := range testFuncs {
			testCaseFlags[i] = customflag.TestCaseFlag(j)
		}
		customflag.ServiceFlag.TestConfig.TestFuncs = testCaseFlags
	}

	os.Exit(m.Run())
}

func TestVersionTestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Version Test Suite")
}

var _ = AfterSuite(func() {
	g := GinkgoT()
	if customflag.ServiceFlag.ClusterConfig.Destroy {
		status, err := factory.DestroyCluster(g)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("cluster destroyed"))
	}
})
