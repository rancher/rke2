package testcase

import (
	"fmt"

	"github.com/rancher/rke2/tests/acceptance/core/service/factory"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestBuildCluster test the creation of a cluster using terraform
func TestBuildCluster(g GinkgoTInterface, destroy bool) {
	cluster := factory.GetCluster(g)

	Expect(cluster.Status).To(Equal("cluster created"))

	shared.PrintFileContents(shared.KubeConfigFile)
	Expect(shared.KubeConfigFile).ShouldNot(BeEmpty())
	Expect(cluster.ServerIPs).ShouldNot(BeEmpty())

	fmt.Println("Server Node IPS:", cluster.ServerIPs)
	fmt.Println("Agent Node IPS:", cluster.AgentIPs)

	if cluster.NumAgents > 0 {
		Expect(cluster.AgentIPs).ShouldNot(BeEmpty())
	} else {
		Expect(cluster.AgentIPs).Should(BeEmpty())
	}
}
