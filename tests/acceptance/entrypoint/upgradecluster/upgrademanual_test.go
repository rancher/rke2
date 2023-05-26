//go:build upgrademanual

package upgradecluster

import (
	"fmt"

	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/core/service/customflag"
	"github.com/rancher/rke2/tests/acceptance/core/testcase"
	"github.com/rancher/rke2/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Test:", func() {

	Context("Build Cluster and validate", func() {

		It("Starts up with no issues", func() {
			testcase.TestBuildCluster(GinkgoT(), false)
		})

		It("Checks Node Status", func() {
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
	})

	It("Verifies ClusterIP Service Pre upgrade", func() {
		testcase.TestServiceClusterIp(true)
	})

	It("Verifies NodePort Service Pre upgrade", func() {
		testcase.TestServiceNodePort(true)
	})

	It("Verifies Ingress Pre upgrade", func() {
		testcase.TestIngress(true)
	})

	It("Verifies Daemonset Pre upgrade", func() {
		testcase.TestDaemonset(true)
	})

	It("Verifies DNS Access Pre upgrade", func() {
		testcase.TestDnsAccess(true)
	})

	It("Upgrade manual", func() {
		_ = testcase.TestUpgradeClusterManually(customflag.InstallType.String())
	})

	It("Checks Node Status pos upgrade", func() {
		testcase.TestNodeStatus(
			assert.NodeAssertReadyStatus(),
			assert.NodeAssertVersionTypeUpgraded(&customflag.InstallType),
		)
	})

	It("Checks Pod Status pos upgrade", func() {
		testcase.TestPodStatus(
			nil,
			assert.PodAssertReady(),
			assert.PodAssertStatus(),
		)
	})

	It("Verifies ClusterIP Service Post upgrade", func() {
		testcase.TestServiceClusterIp(false)
		defer util.ManageWorkload("delete", "clusterip.yaml")
	})

	It("Verifies NodePort Service Post upgrade", func() {
		testcase.TestServiceNodePort(false)
		defer util.ManageWorkload("delete", "nodeport.yaml")
	})

	It("Verifies Ingress Post upgrade", func() {
		testcase.TestIngress(false)
		defer util.ManageWorkload("delete", "ingress.yaml")
	})

	It("Verifies Daemonset Post upgrade", func() {
		testcase.TestDaemonset(false)
		defer util.ManageWorkload("delete", "daemonset.yaml")
	})

	It("Verifies DNS Access Post upgrade", func() {
		testcase.TestDnsAccess(true)
		defer util.ManageWorkload("delete", "dns.yaml")
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
