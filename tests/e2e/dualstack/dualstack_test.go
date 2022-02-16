package validatecluster

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/e2e"
)

// Valid nodeOS: generic/ubuntu2004, opensuse/Leap-15.3.x86_64
var nodeOS = flag.String("nodeOS", "generic/ubuntu2004", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")

// Valid format: RELEASE_VERSION=v1.22.6+rke2r1 or nil for latest commit from master
var installType = flag.String("installType", "", "a string")

func Test_E2EDualStack(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validate DualStack Suite")
}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = Describe("Verify DualStack Configuration", func() {

	It("Starts up with no issues", func() {
		var err error
		serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount, *installType)
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog())
		fmt.Println("CLUSTER CONFIG")
		fmt.Println("OS:", *nodeOS)
		fmt.Println("Server Nodes:", serverNodeNames)
		fmt.Println("Agent Nodes:", agentNodeNames)
		kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred())
	})

	It("Checks Node Status", func() {
		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(kubeConfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"))
			}
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParseNodes(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Checks Pod Status", func() {
		Eventually(func(g Gomega) {
			pods, err := e2e.ParsePods(kubeConfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				if strings.Contains(pod.Name, "helm-install") {
					g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
				} else {
					g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
				}
			}
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParsePods(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Verifies ClusterIP Service", func() {
		_, err := e2e.DeployWorkload("dualstack-clusterip.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() (string, error) {
			cmd := "kubectl get pods -o=name -l app=clusterip-demo --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
			return e2e.RunCommand(cmd)
		}, "240s", "5s").Should(ContainSubstring("test-clusterip"))

		clusterips, _ := e2e.FetchClusterIP(kubeConfigFile, "nginx-clusterip-svc")

		cmd := "\"curl -L --insecure http://" + clusterips + "/name.html\""
		for _, nodeName := range serverNodeNames {
			Expect(e2e.RunCmdOnNode(cmd, nodeName)).Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)
		}
	})

})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentGinkgoTestDescription().Failed
})

var _ = AfterSuite(func() {
	if failed {
		fmt.Println("FAILED!")
	} else {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
