package dualstack

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
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+rke2r1 or nil for latest commit from master

func Test_E2EDualStack(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Validate dualstack Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)
var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify DualStack Configuration", Ordered, func() {

	It("Starts up with no issues", func() {
		var err error
		serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
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
		}, "620s", "5s").Should(Succeed())
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

	It("Verifies that each node has IPv4 and IPv6", func() {
		for _, node := range serverNodeNames {
			cmd := fmt.Sprintf("kubectl get node %s -o jsonpath='{.status.addresses}' --kubeconfig=%s | jq '.[] | select(.type == \"ExternalIP\") | .address'",
				node, kubeConfigFile)
			res, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
			Expect(res).Should(ContainSubstring("10.10.10"))
			Expect(res).Should(ContainSubstring("fd11:decf:c0ff"))
		}
	})
	It("Verifies that each pod has IPv4 and IPv6", func() {
		podIPs, err := e2e.GetPodIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, pod := range podIPs {
			Expect(pod.Ipv4).Should(Or(ContainSubstring("10.10.10"), ContainSubstring("10.42."), ContainSubstring("192.168.")), pod.Name)
			Expect(pod.Ipv6).Should(Or(ContainSubstring("fd11:decf:c0ff"), ContainSubstring("2001:cafe:42")), pod.Name)
		}
	})

	It("Verifies ClusterIP Service", func() {
		_, err := e2e.DeployWorkload("dualstack_clusterip.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() (string, error) {
			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
			return e2e.RunCommand(cmd)
		}, "120s", "5s").Should(ContainSubstring("ds-clusterip-pod"))

		// Checks both IPv4 and IPv6
		clusterips, err := e2e.FetchClusterIP(kubeConfigFile, "ds-clusterip-svc", true)
		Expect(err).NotTo(HaveOccurred())
		for _, ip := range strings.Split(clusterips, ",") {
			if strings.Contains(ip, "::") {
				ip = "[" + ip + "]"
			}
			pods, err := e2e.ParsePods(kubeConfigFile, false)
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				if !strings.HasPrefix(pod.Name, "ds-clusterip-pod") {
					continue
				}
				cmd := fmt.Sprintf("curl -L --insecure http://%s", ip)
				Eventually(func() (string, error) {
					return e2e.RunCmdOnNode(cmd, serverNodeNames[0])
				}, "60s", "5s").Should(ContainSubstring("Welcome to nginx!"), "failed cmd: "+cmd)
			}
		}
	})
	It("Verifies Ingress", func() {
		_, err := e2e.DeployWorkload("dualstack_ingress.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")
		cmd := "kubectl get ingress ds-ingress --kubeconfig=" + kubeConfigFile + " -o jsonpath=\"{.spec.rules[*].host}\""
		hostName, err := e2e.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		nodeIPs, err := e2e.GetNodeIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		for _, node := range nodeIPs {
			cmd := fmt.Sprintf("curl  --header host:%s http://%s/name.html", hostName, node.Ipv4)
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "10s", "2s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
			cmd = fmt.Sprintf("curl  --header host:%s http://[%s]/name.html", hostName, node.Ipv6)
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "5s", "1s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
		}
	})

	It("Verifies NodePort Service", func() {
		_, err := e2e.DeployWorkload("dualstack_nodeport.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		cmd := "kubectl get service ds-nodeport-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
		nodeport, err := e2e.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		nodeIPs, err := e2e.GetNodeIPs(kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, node := range nodeIPs {
			cmd = "curl -L --insecure http://" + node.Ipv4 + ":" + nodeport + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "10s", "1s").Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
			cmd = "curl -L --insecure http://[" + node.Ipv6 + "]:" + nodeport + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "10s", "1s").Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
		}
	})
	It("Verifies podSelector Network Policy", func() {
		_, err := e2e.DeployWorkload("pod_client.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		cmd := "kubectl exec svc/client-curl --kubeconfig=" + kubeConfigFile + " -- curl -m7 ds-clusterip-svc/name.html"
		Eventually(func() (string, error) {
			return e2e.RunCommand(cmd)
		}, "20s", "3s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
		_, err = e2e.DeployWorkload("netpol-fail.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		cmd = "kubectl exec svc/client-curl --kubeconfig=" + kubeConfigFile + " -- curl -m7 ds-clusterip-svc/name.html"
		_, err = e2e.RunCommand(cmd)
		Expect(err).To(HaveOccurred())
		_, err = e2e.DeployWorkload("netpol-work.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		cmd = "kubectl exec svc/client-curl --kubeconfig=" + kubeConfigFile + " -- curl -m7 ds-clusterip-svc/name.html"
		Eventually(func() (string, error) {
			return e2e.RunCommand(cmd)
		}, "20s", "3s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
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
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
