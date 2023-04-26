package testfunctions

import (
	"fmt"
	"strings"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/terraform/core/assert"
	"github.com/rancher/rke2/tests/terraform/core/factory"
	"github.com/rancher/rke2/tests/terraform/shared/util"
)

func TestTFBuildCluster(t ginkgo.GinkgoTInterface) {
	status, err := factory.BuildCluster(t, false)

	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(status).To(gomega.Equal("cluster created"))

	util.PrintFileContents(util.KubeConfigFile)
	gomega.Expect(util.KubeConfigFile).ShouldNot(gomega.BeEmpty())
	gomega.Expect(util.ServerIPs).ShouldNot(gomega.BeEmpty())

	fmt.Println("Server Node IPS:", util.ServerIPs)
	fmt.Println("Agent Node IPS:", util.AgentIPs)

	if util.NumAgents > 0 {
		gomega.Expect(util.AgentIPs).ShouldNot(gomega.BeEmpty())
	} else {
		gomega.Expect(util.AgentIPs).Should(gomega.BeEmpty())
	}
}

func TestTFNodeAndPodStatus(
	g ginkgo.GinkgoTInterface,
	nodeAssertReadyStatus assert.NodeAssertFunc,
	nodeAssertVersion assert.NodeAssertFunc,
	podAssertRestarts assert.PodAssertFunc,
	podAssertReady assert.PodAssertFunc,
) {
	defer func() {
		_, err := util.Nodes(true)
		if err != nil {
			fmt.Println("Error retrieving nodes: ", err)
		}
		_, err = util.Pods(true)
		if err != nil {
			fmt.Println("Error retrieving pods: ", err)
		}
	}()

	fmt.Printf("\nFetching node status\n")

	expectedNodeCount := util.NumServers + util.NumAgents
	gomega.Eventually(func(g gomega.Gomega) {
		nodes, err := util.Nodes(false)

		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(len(nodes)).To(gomega.Equal(expectedNodeCount),
			"Number of nodes should match the spec")

		for _, node := range nodes {
			if nodeAssertReadyStatus != nil {
				nodeAssertReadyStatus(g, node)
			}
			if nodeAssertVersion != nil {
				nodeAssertVersion(g, node)
			}
		}
	}, "600s", "5s").Should(gomega.Succeed())

	fmt.Printf("\nFetching pod status\n")

	gomega.Eventually(func(g gomega.Gomega) {
		pods, err := util.Pods(false)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		for _, pod := range pods {
			if strings.Contains(pod.Name, "helm-install") {
				g.Expect(pod.Status).Should(gomega.Equal("Completed"), pod.Name)
			} else if strings.Contains(pod.Name, "apply") {
				g.Expect(pod.Status).Should(gomega.SatisfyAny(
					gomega.ContainSubstring("Error"),
					gomega.Equal("Completed"),
				), pod.Name)
			} else {
				g.Expect(pod.Status).Should(gomega.Equal("Running"), pod.Name)
				if podAssertRestarts != nil {
					podAssertRestarts(g, pod)
				}
				if podAssertReady != nil {
					podAssertReady(g, pod)
				}
			}
		}
	}, "600s", "5s").Should(gomega.Succeed())
}
