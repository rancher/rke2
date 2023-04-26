package createcluster

import (
	"fmt"

	"github.com/rancher/rke2/tests/terraform/core/assert"
	"github.com/rancher/rke2/tests/terraform/core/factory"
	"github.com/rancher/rke2/tests/terraform/shared/util"
	testfunctions2 "github.com/rancher/rke2/tests/terraform/testfunctions"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test:", func() {

	Context("Build Cluster:", func() {

		It("Start Up with no issues", func() {
			testfunctions2.TestTFBuildCluster(GinkgoT())
		})

		It("Checks Node and Pod Status", func() {
			testfunctions2.TestTFNodeAndPodStatus(GinkgoT(), assert.NodeAssertReadyStatus(),
				nil,
				assert.PodAssertRestarts(), assert.PodAssertReadyStatus())
		})

		It("Verifies ClusterIP Service", func() {
			testfunctions2.TestTFServiceClusterIp(GinkgoT(), true)
			defer util.ManageWorkload("delete", "clusterip.yaml")
		})

		It("Verifies NodePort Service", func() {
			testfunctions2.TestTFServiceNodePort(GinkgoT(), true)
			defer util.ManageWorkload("delete", "nodeport.yaml")
		})

		It("Verifies Ingress", func() {
			testfunctions2.TestTFIngress(GinkgoT(), true)
			defer util.ManageWorkload("delete", "ingress.yaml")
		})

		It("Verifies Daemonset", func() {
			testfunctions2.TestTFDaemonset(GinkgoT(), true)
			defer util.ManageWorkload("delete", "daemonset.yaml")
		})

		It("Verifies dns access", func() {
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

var _ = AfterSuite(func() {
	if *util.Destroy {
		status, err := factory.BuildCluster(GinkgoT(), *util.Destroy)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("cluster destroyed"))
	}
})
