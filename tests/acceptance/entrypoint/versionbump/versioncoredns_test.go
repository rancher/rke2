//go:build coredns

package versionbump

import (
	"fmt"

	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/core/service/customflag"
	"github.com/rancher/rke2/tests/acceptance/core/service/template"
	"github.com/rancher/rke2/tests/acceptance/core/testcase"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("VersionTemplate Upgrade:", func() {

	It("Start Up with no issues", func() {
		testcase.TestBuildCluster(GinkgoT(), false)
	})

	It("Validate Nodes", func() {
		testcase.TestNodeStatus(
			assert.NodeAssertReadyStatus(),
			nil,
		)
	})

	It("Validate Pods", func() {
		testcase.TestPodStatus(
			assert.PodAssertRestart(),
			assert.PodAssertReady(),
			assert.PodAssertStatus(),
		)
	})

	It("Test Bump version CoreDNS", func() {
		template.VersionTemplate(template.VersionTestTemplate{
			Description: "Test CoreDNS bump",
			TestCombination: &template.RunCmd{
				RunOnHost: []template.TestMap{
					{
						Cmd: "kubectl get deploy rke2-coredns-rke2-coredns -n kube-system -o " +
							"jsonpath='{.spec.template.spec.containers[?(@.name==\"coredns\")].image}'",
						ExpectedValue:        template.TestMapFlag.ExpectedValueHost,
						ExpectedValueUpgrade: template.TestMapFlag.ExpectedValueUpgradedHost,
					},
				},
			},
			InstallUpgrade: customflag.ServiceFlag.InstallUpgrade,
			TestConfig: &template.TestConfig{
				TestFunc:       template.TestCase(customflag.ServiceFlag.TestCase.TestFunc),
				DeployWorkload: customflag.ServiceFlag.TestCase.DeployWorkload,
			},
		})
	})
})

var _ = AfterEach(func() {
	if CurrentSpecReport().Failed() {
		fmt.Printf("\nFAILED! %s\n", CurrentSpecReport().FullText())
	} else {
		fmt.Printf("\nPASSED! %s\n", CurrentSpecReport().FullText())
	}
})
