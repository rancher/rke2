package calico_ebpf

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

var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 1, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built RKE2")

func Test_E2ECalicoEBPF(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Validate dualstack in Calico eBPF Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig
var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify DualStack in Calico eBPF configuration", Ordered, func() {
	It("Starts up with no issues", func() {
		var err error
		if *local {
			tc, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, *agentCount)
		} else {
			tc, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
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
		}, "600s", "5s").Should(Succeed())
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

	It("Verifies that each node has IPv4 and IPv6", func() {
		for _, node := range tc.Servers {
			cmd := fmt.Sprintf("kubectl get node %s -o jsonpath='{.status.addresses}' --kubeconfig=%s | jq '.[] | select(.type == \"ExternalIP\") | .address'",
				node.Name, tc.KubeconfigFile)
			res, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
			Expect(res).Should(ContainSubstring("10.10.10"))
			Expect(res).Should(ContainSubstring("fd11:decf:c0ff"))
		}
	})

	It("Verifies ClusterIP Service", func() {
		_, err := tc.DeployWorkload("dualstack_clusterip.yaml")
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() (string, error) {
			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
			return e2e.RunCommand(cmd)
		}, "120s", "5s").Should(ContainSubstring("ds-clusterip-pod"))

		// Checks both IPv4 and IPv6
		clusterips, err := e2e.FetchClusterIP(tc.KubeconfigFile, "ds-clusterip-svc", true)
		Expect(err).NotTo(HaveOccurred())
		for _, ip := range strings.Split(clusterips, ",") {
			if strings.Contains(ip, "::") {
				ip = "[" + ip + "]"
			}
			pods, err := e2e.ParsePods(tc.KubeconfigFile, false)
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				if !strings.HasPrefix(pod.Name, "ds-clusterip-pod") {
					continue
				}
				cmd := fmt.Sprintf("curl -L --insecure http://%s", ip)
				Eventually(func() (string, error) {
					return tc.Servers[0].RunCmdOnNode(cmd)
				}, "60s", "5s").Should(ContainSubstring("Welcome to nginx!"), "failed cmd: "+cmd)
			}
		}
	})

	It("Verifies internode connectivity", func() {
		_, err := tc.DeployWorkload("pod_client.yaml")
		Expect(err).NotTo(HaveOccurred())

		// Wait for the pod_client to have an IP
		Eventually(func() string {
			ips, _ := e2e.PodIPsUsingLabel(tc.KubeconfigFile, "app=client")
			return ips[0].Ipv4
		}, "40s", "5s").Should(ContainSubstring("10.42"), "failed getClientIPs")

		clientIPs, err := e2e.PodIPsUsingLabel(tc.KubeconfigFile, "app=client")
		Expect(err).NotTo(HaveOccurred())
		for _, ip := range clientIPs {
			cmd := "kubectl exec svc/client-wget --kubeconfig=" + tc.KubeconfigFile + " -- wget -T7 -O - " + ip.Ipv4 + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "20s", "3s").Should(ContainSubstring("client-deployment"), "failed cmd: "+cmd)
		}
	})

	It("Verifies Ingress", func() {
		_, err := tc.DeployWorkload("dualstack_ingress.yaml")
		Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")
		cmd := "kubectl get ingress ds-ingress --kubeconfig=" + tc.KubeconfigFile + " -o jsonpath=\"{.spec.rules[*].host}\""
		hostName, err := e2e.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		nodeIPs, err := e2e.GetNodeIPs(tc.KubeconfigFile)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		for _, node := range nodeIPs {
			cmd := fmt.Sprintf("curl  --header host:%s http://%s/name.html", hostName, node.Ipv4)
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "30s", "2s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
			cmd = fmt.Sprintf("curl  --header host:%s http://[%s]/name.html", hostName, node.Ipv6)
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "10s", "1s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
		}
	})

	It("Verifies NodePort Service", func() {
		_, err := tc.DeployWorkload("dualstack_nodeport.yaml")
		Expect(err).NotTo(HaveOccurred())
		cmd := "kubectl get service ds-nodeport-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
		nodeport, err := e2e.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		nodeIPs, err := e2e.GetNodeIPs(tc.KubeconfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, node := range nodeIPs {
			cmd = "curl -L --insecure http://" + node.Ipv4 + ":" + nodeport + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "30s", "1s").Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
			cmd = "curl -L --insecure http://[" + node.Ipv6 + "]:" + nodeport + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "10s", "1s").Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
		}
	})

	It("Verifies there are no required iptables", func() {
		// Check that there are no iptables rules with KUBE-SVC
		cmdiptables := "sudo iptables-save | grep -e 'KUBE-SVC' | wc -l"
		for _, server := range tc.Servers {
			res, err := server.RunCmdOnNode(cmdiptables)
			Expect(err).NotTo(HaveOccurred(), res)
			Expect(res).Should(ContainSubstring("0"))
		}
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
