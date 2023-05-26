package createcluster

import (
	"fmt"

	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/core/testcase"
	"github.com/rancher/rke2/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Test:", func() {

	Context("Build Cluster:", func() {

		It("Start Up with no issues", func() {
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

		It("Verifies ClusterIP Service", func() {
			testcase.TestServiceClusterIp(true)
			defer util.ManageWorkload("delete", "clusterip.yaml")
		})

		It("Verifies NodePort Service", func() {
			testcase.TestServiceNodePort(true)
			defer util.ManageWorkload("delete", "nodeport.yaml")
		})

		It("Verifies Ingress", func() {
			testcase.TestIngress(true)
			defer util.ManageWorkload("delete", "ingress.yaml")
		})

		It("Verifies Daemonset", func() {
			testcase.TestDaemonset(true)
			defer util.ManageWorkload("delete", "daemonset.yaml")
		})

		It("Verifies dns access", func() {
			testcase.TestDnsAccess(true)
			defer util.ManageWorkload("delete", "dnsutils.yaml")
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
