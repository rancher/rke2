package template

import (
	"fmt"
	"sync"

	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
)

// processTestCombination runs the tests per ips using CmdOnNode and CmdOnHost validation
// it will spawn a go routine per ip
func processTestCombination(resultChan chan error, wg *sync.WaitGroup, ips []string, testCombination RunCmd) {
	for _, ip := range ips {
		if testCombination.RunOnHost != nil {
			for _, test := range testCombination.RunOnHost {
				wg.Add(1)
				go func(ip string, cmd, expectedValue, expectedValueUpgraded string) {
					defer wg.Done()
					defer GinkgoRecover()
					processOnHost(resultChan, ip, cmd, expectedValue)
				}(ip, test.Cmd, test.ExpectedValue, test.ExpectedValueUpgrade)
			}
		}

		if testCombination.RunOnNode != nil {
			for _, test := range testCombination.RunOnNode {
				wg.Add(1)
				go func(ip string, cmd, expectedValue string) {
					defer wg.Done()
					defer GinkgoRecover()
					processOnNode(resultChan, ip, cmd, expectedValue)
				}(ip, test.Cmd, test.ExpectedValue)
			}
		}
	}
}

// processOnNode runs the test on the node calling ValidateOnNode
func processOnNode(resultChan chan error, ip, cmd, expectedValue string) {
	if expectedValue == "" {
		err := fmt.Errorf("\nexpected value should be sent to node")
		fmt.Println("error:", err)
		resultChan <- err
		close(resultChan)
		return
	}

	version := shared.GetRke2Version()
	fmt.Printf("\n Checking version: %s on ip: %s \n "+
		"Command: %s, \n Expected Value: %s", version, ip, cmd, expectedValue)

	joinedCmd := joinCommands(cmd, "")

	err := assert.ValidateOnNode(
		ip,
		joinedCmd,
		expectedValue,
	)
	if err != nil {
		resultChan <- err
		close(resultChan)
		return
	}
}

// processOnHost runs the test on the host calling ValidateOnHost
func processOnHost(resultChan chan error, ip, cmd, expectedValue string) {
	if expectedValue == "" {
		err := fmt.Errorf("\nexpected value should be sent to host")
		fmt.Println("error:", err)
		resultChan <- err
		close(resultChan)
		return
	}

	kubeconfigFlag := " --kubeconfig=" + shared.KubeConfigFile
	fullCmd := joinCommands(cmd, kubeconfigFlag)

	version := shared.GetRke2Version()
	fmt.Printf("\n Checking version: %s on ip: %s \n "+
		"Command: %s, \n Expected Value: %s", version, ip, fullCmd, expectedValue)

	err := assert.ValidateOnHost(
		fullCmd,
		expectedValue,
	)
	if err != nil {
		resultChan <- err
		close(resultChan)
		return
	}
}
