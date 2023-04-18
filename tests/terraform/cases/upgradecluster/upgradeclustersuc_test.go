package upgradecluster

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	"github.com/rancher/rke2/tests/terraform/core/assert"
	"github.com/rancher/rke2/tests/terraform/core/factory"
	"github.com/rancher/rke2/tests/terraform/shared/util"
	testfunctions2 "github.com/rancher/rke2/tests/terraform/testfunctions"
)

var _ = Describe("Upgrade Tests:", func() {

	Context("Build Cluster:", func() {

		It("Starts up with no issues", func() {
			testfunctions2.TestTFBuildCluster(GinkgoT())
		})

		It("Checks Node and Pod Status", func() {
			testfunctions2.TestTFNodeAndPodStatus(GinkgoT(), assert.NodeAssertReadyStatus(),
				nil,
				assert.PodAssertRestarts(), assert.PodAssertReadyStatus())
		})

		Context("Preupgrade Validations:", func() {

			It("Verifies ClusterIP Service Preupgrade", func() {
				testfunctions2.TestTFServiceClusterIp(GinkgoT(), true)
			})

			It("Verifies NodePort Service Preupgrade", func() {
				testfunctions2.TestTFServiceNodePort(GinkgoT(), true)
			})

			It("Verifies Ingress Preupgrade", func() {
				testfunctions2.TestTFIngress(GinkgoT(), true)
			})

			It("Verifies Daemonset Preupgrade", func() {
				testfunctions2.TestTFDaemonset(GinkgoT(), true)
			})

			It("Verifies DNS Access Preupgrade", func() {
				testfunctions2.TestDnsAccess(GinkgoT(), true)
			})
		})

		Context("Upgrade via SUC:", func() {

			It("Verifies Upgrade", func() {
				_ = factory.UpgradeClusterSUC(*util.UpgradeVersion)
				testfunctions2.TestTFNodeAndPodStatus(GinkgoT(), assert.NodeAssertReadyStatus(),
					assert.NodeAssertVersionUpgraded(),
					nil, assert.PodAssertReadyStatus())
			})
		})
	})

	Context("Postupgrade Validations:", func() {

		It("Verifies ClusterIP Service Postupgrade", func() {
			testfunctions2.TestTFServiceClusterIp(GinkgoT(), false)
			defer util.ManageWorkload("delete", "clusterip.yaml")
		})

		It("Verifies NodePort Service Postupgrade", func() {
			testfunctions2.TestTFServiceNodePort(GinkgoT(), false)
			defer util.ManageWorkload("delete", "nodeport.yaml")
		})

		It("Verifies Ingress Postupgrade", func() {
			testfunctions2.TestTFIngress(GinkgoT(), false)
			defer util.ManageWorkload("delete", "ingress.yaml")
		})

		It("Verifies Daemonset Postupgrade", func() {
			testfunctions2.TestTFDaemonset(GinkgoT(), false)
			defer util.ManageWorkload("delete", "daemonset.yaml")
		})

		It("Verifies DNS Access Postupgrade", func() {
			testfunctions2.TestDnsAccess(GinkgoT(), true)
			defer util.ManageWorkload("delete", "dns.yaml")
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
