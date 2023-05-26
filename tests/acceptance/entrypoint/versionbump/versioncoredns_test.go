//go:build coredns

package versionbump

import (
	"fmt"

	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/core/service/customflag"
	"github.com/rancher/rke2/tests/acceptance/core/service/template"
	"github.com/rancher/rke2/tests/acceptance/core/testcase"
	"github.com/rancher/rke2/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("VersionTemplate Upgrade:", func() {

	It("Start Up with no issues", func() {
		testcase.TestBuildCluster(GinkgoT(), false)
	})

	It("Check Node Status", func() {
		testcase.TestNodeStatus(
			assert.NodeAssertReadyStatus(),
			nil,
		)
	})

	It("Checks Pod Status", func() {
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
						Cmd:                  util.GetCoreDNSdeployImage,
						ExpectedValue:        ExpectedValueHost,
						ExpectedValueUpgrade: ExpectedValueUpgradedHost,
					},
				},
			},
			InstallUpgrade: customflag.InstallUpgradeFlag,
			TestConfig: &template.TestConfig{
				TestFunc:       testcase.TestCoredns,
				DeployWorkload: true,
			},
		})
	})
})

var _ = BeforeEach(func() {
	if *util.Destroy {
		Skip("Cluster is being Deleted")
	}
})

var _ = AfterEach(func() {
	if CurrentSpecReport().Failed() {
		fmt.Printf("\nFAILED! %s\n", CurrentSpecReport().FullText())
	} else {
		fmt.Printf("\nPASSED! %s\n", CurrentSpecReport().FullText())
	}
})
