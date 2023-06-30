package createcluster

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	"github.com/rancher/rke2/tests/acceptance/core/testcase"
	"github.com/rancher/rke2/tests/acceptance/shared"
)

var _ = Describe("Test:", func() {

	It("Start Up with no issues", func() {
		testcase.TestBuildCluster(GinkgoT(), false)
	})

	// It("Validate Nodes", func() {
	// 	testcase.TestNodeStatus(
	// 		assert.NodeAssertReadyStatus(),
	// 		nil,
	// 	)
	// })
	//
	// It("Validate Pods", func() {
	// 	testcase.TestPodStatus(
	// 		assert.PodAssertRestart(),
	// 		assert.PodAssertReady(),
	// 		assert.PodAssertStatus(),
	// 	)
	// })

	// It("Verifies ClusterIP Service", func() {
	// 	testcase.TestServiceClusterIp(true)
	// 	defer shared.ManageWorkload("delete", "clusterip.yaml")
	// })
	//
	// It("Verifies NodePort Service", func() {
	// 	testcase.TestServiceNodePort(true)
	// 	defer shared.ManageWorkload("delete", "nodeport.yaml")
	// })
	//
	It("Verifies Ingress", func() {
		testcase.TestIngress(true)
		defer shared.ManageWorkload("delete", "ingress.yaml")
	})

	It("Verifies Daemonset", func() {
		testcase.TestDaemonset(true)
		defer shared.ManageWorkload("delete", "daemonset.yaml")
	})

	It("Verifies dns access", func() {
		testcase.TestDnsAccess(true)
		defer shared.ManageWorkload("delete", "dnsutils.yaml")
	})
})

var _ = AfterEach(func() {
	if CurrentSpecReport().Failed() {
		fmt.Printf("\nFAILED! %s\n", CurrentSpecReport().FullText())
	} else {
		fmt.Printf("\nPASSED! %s\n", CurrentSpecReport().FullText())
	}
})
