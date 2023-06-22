package testcase

import (
	"fmt"

	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/core/service/factory"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// TestNodeStatus test the status of the nodes in the cluster using 2 custom assert functions
func TestNodeStatus(
	nodeAssertReadyStatus assert.NodeAssertFunc,
	nodeAssertVersion assert.NodeAssertFunc,
) {
	cluster := factory.GetCluster(GinkgoT())
	fmt.Println("\n***********************************\n")
	fmt.Printf("Checking node status\n")
	defer func() {
		fmt.Println("\n***********************************\n")
		fmt.Printf("Cluster nodes\n")
		_, err := shared.Nodes(true)
		if err != nil {
			fmt.Println("Error retrieving pods: ", err)
		}
	}()

	expectedNodeCount := cluster.NumServers + cluster.NumAgents + cluster.NumWinAgents
	Eventually(func(g Gomega) {
		nodes, err := shared.Nodes(false)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(len(nodes)).To(Equal(expectedNodeCount),
			"Number of nodes should match the spec")

		for _, node := range nodes {
			if nodeAssertReadyStatus != nil {
				nodeAssertReadyStatus(g, node)
			}
			if nodeAssertVersion != nil {
				nodeAssertVersion(g, node)
			}
		}
	}, "800s", "3s").Should(Succeed())
}

// Deploys services in the cluster and validates communication between nodes
func TestInternodeConnectivity() {
	shared.ManageWorkload("apply","pod_client.yaml")
	assert.ValidatePodIPByLabel("app=client","10.42")
	shared.ManageWorkload("apply","windows_app_deployment.yaml")
	assert.ValidatePodIPByLabel("app=windows-app","10.42")
	defer shared.ManageWorkload("delete","pod_client.yaml")
	defer shared.ManageWorkload("delete","windows_app_deployment.yaml")
	testServiceCrossNodeCommunication()

}

func testServiceCrossNodeCommunication() {
	// Test Linux -> Windows communication
	cmd := "kubectl exec svc/client-curl --kubeconfig=" + shared.KubeConfigFile + " -- curl -m7 windows-app-svc:3000"
	Eventually(func() (string, error) {
		return shared.RunCommandHost(cmd)
	}, "120s", "3s").Should(ContainSubstring("Welcome to PSTools for K8s Debugging"), fmt.Errorf("failed cmd: %s",cmd))

	// Test Windows -> Linux communication
	cmd = "kubectl exec svc/windows-app-svc --kubeconfig=" + shared.KubeConfigFile + " -- curl -m7 client-curl:8080"
	Eventually(func() (string, error) {
		return shared.RunCommandHost(cmd)
	}, "120s", "3s").Should(ContainSubstring("Welcome to nginx!"), fmt.Errorf("failed cmd: %s",cmd))
}

func TestSonobuoyMixedOS() {
	shared.InstallSonobuoy()
	cmd := `sonobuoy run --kubeconfig=` + shared.KubeConfigFile + ` --plugin my-sonobuoy-plugins/mixed-workload-e2e/mixed-workload-e2e.yaml --aggregator-node-selector kubernetes.io/os:linux --wait`
	fmt.Println(cmd)
	res, err := shared.RunCommandHost(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed output: " + res)
	cmd = `sonobuoy retrieve --kubeconfig=`+ shared.KubeConfigFile
	testResultTar, err := shared.RunCommandHost(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed cmd: "+ cmd)
	cmd = "sonobuoy results " + testResultTar
	res, err = shared.RunCommandHost(cmd)
	Expect(err).NotTo(HaveOccurred(), "failed cmd: "+ cmd)
	Expect(res).Should(ContainSubstring("Plugin: mixed-workload-e2e\nStatus: passed\n"))
}