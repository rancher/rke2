package template

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rancher/rke2/tests/acceptance/core/testcase"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
)

// upgradeVersion upgrades the version of RKE2 and updates the expected values
func upgradeVersion(template VersionTestTemplate, version string) error {
	err := testcase.TestUpgradeClusterManually(version)
	if err != nil {
		return err
	}

	for i := range template.TestCombination.RunOnNode {
		template.TestCombination.RunOnNode[i].ExpectedValue =
			template.TestCombination.RunOnNode[i].ExpectedValueUpgrade
	}

	for i := range template.TestCombination.RunOnHost {
		template.TestCombination.RunOnHost[i].ExpectedValue =
			template.TestCombination.RunOnHost[i].ExpectedValueUpgrade
	}

	return nil
}

// checkVersion checks the version of RKE2 by calling processTestCombination
func checkVersion(v VersionTestTemplate) error {
	ips, err := getIPs()
	if err != nil {
		GinkgoT().Errorf("Failed to get IPs: %s", err)
	}

	var wg sync.WaitGroup
	errorChanList := make(
		chan error,
		len(ips)*(len(v.TestCombination.RunOnHost)+len(v.TestCombination.RunOnNode)),
	)

	processTestCombination(errorChanList, &wg, ips, *v.TestCombination)

	wg.Wait()
	close(errorChanList)

	for chanErr := range errorChanList {
		if chanErr != nil {
			return chanErr
		}
	}

	if v.TestConfig != nil {
		TestCaseWrapper(v)
	}

	return nil
}

// joinCommands joins the first command with some arg
func joinCommands(cmd, kubeconfigFlag string) string {
	cmds := strings.Split(cmd, ",")
	joinedCmd := cmds[0] + kubeconfigFlag

	if len(cmds) > 1 {
		secondCmd := strings.Join(cmds[1:], ",")
		joinedCmd += " " + secondCmd
	}

	return joinedCmd
}

// getIPs gets the IPs of the nodes
func getIPs() (ips []string, err error) {
	ips = shared.FetchNodeExternalIP()
	return ips, nil
}

// AddTestCase returns the test case based on the name to be used as customflag.
func AddTestCase(name string) (TestCase, error) {
	if name == "" {
		return func(deployWorkload bool) {}, nil
	}

	testCase := map[string]TestCase{
		"TestDaemonset":        testcase.TestDaemonset,
		"TestIngress":          testcase.TestIngress,
		"TestDnsAccess":        testcase.TestDnsAccess,
		"TestServiceClusterIp": testcase.TestServiceClusterIp,
		"TestServiceNodePort":  testcase.TestServiceNodePort,
		"TestCoredns":          testcase.TestCoredns,
	}

	if test, ok := testCase[name]; ok {
		return test, nil
	} else {
		return nil, fmt.Errorf("invalid test case name")
	}
}
