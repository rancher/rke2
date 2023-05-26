package testcase

import (
	"fmt"

	"github.com/rancher/rke2/tests/acceptance/core/service/factory"
	"github.com/rancher/rke2/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestBuildCluster test the creation of a cluster using terraform
func TestBuildCluster(g GinkgoTInterface, destroy bool) {
	status, err := factory.BuildCluster(g, destroy)
	if err != nil {
		return
	}
	Expect(status).To(Equal("cluster created"))

	util.PrintFileContents(util.KubeConfigFile)
	Expect(util.KubeConfigFile).ShouldNot(BeEmpty())
	Expect(util.ServerIPs).ShouldNot(BeEmpty())

	fmt.Println("Server Node IPS:", util.ServerIPs)
	fmt.Println("Agent Node IPS:", util.AgentIPs)

	if util.NumAgents > 0 {
		Expect(util.AgentIPs).ShouldNot(BeEmpty())
	} else {
		Expect(util.AgentIPs).Should(BeEmpty())
	}
}
