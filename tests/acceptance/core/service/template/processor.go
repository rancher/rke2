package template

import (
	"fmt"
	"strings"
	"sync"

	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
)

// processTestCombination runs the tests per ips using processOnNode and processOnHost validation.
//
// it will spawn a go routine per testCombination and ip.
func processTestCombination(resultChan chan error, wg *sync.WaitGroup, ips []string, testCombination RunCmd) {
	for _, ip := range ips {
		if testCombination.Run != nil {
			for _, testMap := range testCombination.Run {
				cmds := strings.Split(testMap.Cmd, ",")
				expectedValues := strings.Split(testMap.ExpectedValue, ",")
				if len(cmds) != len(expectedValues) {
					resultChan <- fmt.Errorf("mismatched length commands x expected values")
					return
				}

				for i := range cmds {
					cmd := cmds[i]
					expectedValue := expectedValues[i]
					if strings.Contains(cmd, "kubectl") || strings.Contains(cmd, "helm") {
						wg.Add(1)
						go func(ip string, cmd, expectedValue string) {
							defer wg.Done()
							defer GinkgoRecover()
							processOnHost(resultChan, ip, cmd, expectedValue)
						}(ip, cmd, expectedValue)
					} else {
						wg.Add(1)
						go func(ip string, cmd, expectedValue string) {
							defer wg.Done()
							defer GinkgoRecover()
							processOnNode(resultChan, ip, cmd, expectedValue)
						}(ip, cmd, expectedValue)
					}
				}
			}
		}
	}
}

// processOnNode runs the test on the node calling ValidateOnNode.
func processOnNode(resultChan chan error, ip, cmd, expectedValue string) {
	if expectedValue == "" {
		err := fmt.Errorf("\nexpected value should be sent to node")
		fmt.Println("error:", err)
		resultChan <- err
		close(resultChan)
		return
	}

	version := shared.GetRke2Version()
	fmt.Printf("\n---------------------\n"+
		"Version Checked: %s\n"+
		"IP Address: %s\n"+
		"Command Executed: %s\n"+
		"Execution Location: Node\n"+
		"Expected Value: %s\n---------------------\n",
		version, ip, cmd, expectedValue)

	cmds := strings.Split(cmd, ",")
	for _, c := range cmds {
		err := assert.ValidateOnNode(
			ip,
			c,
			expectedValue,
		)
		if err != nil {
			resultChan <- err
			close(resultChan)
			return
		}
	}
}

// processOnHost runs the test on the host calling ValidateOnHost.
func processOnHost(resultChan chan error, ip, cmd, expectedValue string) {
	if expectedValue == "" {
		err := fmt.Errorf("\nexpected value should be sent to host")
		fmt.Println("error:", err)
		resultChan <- err
		close(resultChan)
		return
	}

	kubeconfigFlag := " --kubeconfig=" + shared.KubeConfigFile
	fullCmd := shared.JoinCommands(cmd, kubeconfigFlag)

	version := shared.GetRke2Version()
	fmt.Printf("\n---------------------\n"+
		"Version Checked: %s\n"+
		"IP Address: %s\n"+
		"Command Executed: %s\n"+
		"Execution Location: Host\n"+
		"Expected Value: %s\n---------------------\n",
		version, ip, cmd, expectedValue)

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
