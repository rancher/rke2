package mixedos

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

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "operating system for linux nodes")
var serverCount = flag.Int("serverCount", 1, "number of server nodes")
var linuxAgentCount = flag.Int("linuxAgentCount", 1, "number of linux agent nodes")
var windowsAgentCount = flag.Int("windowsAgentCount", 1, "number of windows agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built RKE2")

func Test_E2EMixedOSValidation(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Validate MixedOS Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)
var _ = Describe("Verify Basic Cluster Creation", Ordered, func() {

	It("Starts up with no issues", func() {
		var err error
		if *local {
			tc, err = e2e.CreateLocalMixedCluster(*nodeOS, *serverCount, *linuxAgentCount, *windowsAgentCount)
		} else {
			tc, err = e2e.CreateMixedCluster(*nodeOS, *serverCount, *linuxAgentCount, *windowsAgentCount)
		}
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
		By("CLUSTER CONFIG")
		By("OS: " + *nodeOS)
		By(tc.Status())
		tc.KubeconfigFile, err = e2e.GenKubeConfigFile(tc.Servers[0])
		Expect(err).NotTo(HaveOccurred())
	})

	It("Checks Node Status", func() {
		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"))
			}
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParseNodes(tc.KubeconfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Checks Pod Status", func() {
		Eventually(func(g Gomega) {
			pods, err := e2e.ParsePods(tc.KubeconfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				if strings.Contains(pod.Name, "helm-install") {
					g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
				} else {
					g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
				}
			}
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParsePods(tc.KubeconfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})
	It("Verifies internode connectivity over the vxlan tunnel", func() {
		_, err := tc.DeployWorkload("pod_client.yaml")
		Expect(err).NotTo(HaveOccurred())

		_, err = tc.DeployWorkload("windows_app_deployment.yaml")
		Expect(err).NotTo(HaveOccurred())

		// Wait for the pod_client pods to have an IP
		Eventually(func() string {
			ips, _ := e2e.PodIPsUsingLabel(tc.KubeconfigFile, "app=client")
			return ips[0].Ipv4
		}, "120s", "10s").Should(ContainSubstring("10.42"), "failed getClientIPs")

		// Wait for the windows_app_deployment pods to have an IP (We must wait 250s because it takes time)
		Eventually(func() string {
			ips, _ := e2e.PodIPsUsingLabel(tc.KubeconfigFile, "app=windows-app")
			return ips[0].Ipv4
		}, "620s", "10s").Should(ContainSubstring("10.42"), "failed getClientIPs")

		// Test Linux -> Windows communication
		cmd := "kubectl exec svc/client-wget --kubeconfig=" + tc.KubeconfigFile + " -- wget -T7 -O - windows-app-svc:3000"
		Eventually(func() (string, error) {
			return e2e.RunCommand(cmd)
		}, "120s", "3s").Should(ContainSubstring("Welcome to PSTools for K8s Debugging"), "failed cmd: "+cmd)

		// Test Windows -> Linux communication
		cmd = "kubectl exec svc/windows-app-svc --kubeconfig=" + tc.KubeconfigFile + " -- curl -m7 client-wget:8080"
		Eventually(func() (string, error) {
			return e2e.RunCommand(cmd)
		}, "20s", "3s").Should(ContainSubstring("Welcome to nginx!"), "failed cmd: "+cmd)
	})
	It("Runs the mixed os sonobuoy plugin", func() {
		cmd := "sonobuoy run --kubeconfig=/etc/rancher/rke2/rke2.yaml --plugin my-sonobuoy-plugins/mixed-workload-e2e/mixed-workload-e2e.yaml --aggregator-node-selector kubernetes.io/os:linux --wait"
		res, err := tc.Servers[0].RunCmdOnNode(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed output:"+res)
		cmd = "sonobuoy retrieve --kubeconfig=/etc/rancher/rke2/rke2.yaml"
		testResultTar, err := tc.Servers[0].RunCmdOnNode(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		cmd = "sonobuoy results " + testResultTar
		res, err = tc.Servers[0].RunCmdOnNode(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		Expect(res).Should(ContainSubstring("Plugin: mixed-workload-e2e\nStatus: passed\n"))
	})

})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed && !*ci {
		fmt.Println("FAILED!")
	} else {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
