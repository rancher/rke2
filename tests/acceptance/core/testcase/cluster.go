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
	Expect(shared.KubeConfigFile).ShouldNot(BeEmpty())
	Expect(cluster.ServerIPs).ShouldNot(BeEmpty())

	fmt.Println("\n***********************************\n")
	fmt.Println("Kubeconfig file:\n")
	shared.PrintFileContents(shared.KubeConfigFile)
	fmt.Println("\n***********************************\n")
	fmt.Println("Base64 Encoded Kubeconfig file:")
	shared.PrintBase64Encoded(shared.KubeConfigFile)
	fmt.Println("\n***********************************\n")
	fmt.Println("Server Node IPS:", cluster.ServerIPs)
	fmt.Println("Agent Node IPS:", cluster.AgentIPs)
	fmt.Println("Windows Agent Node IPS:", cluster.WinAgentIPs)

	if cluster.NumAgents > 0 {
		Expect(cluster.AgentIPs).ShouldNot(BeEmpty())
	} else {
		Expect(cluster.AgentIPs).Should(BeEmpty())
	}

	if cluster.NumWinAgents > 0 {
		Expect(cluster.WinAgentIPs).ShouldNot(BeEmpty())
	} else {
		Expect(cluster.WinAgentIPs).Should(BeEmpty())
	}
}
