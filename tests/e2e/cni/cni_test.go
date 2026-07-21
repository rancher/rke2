package nightly_cni

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
var filter = flag.String("filter", "iptables", "iptables or nftables")
var cni = flag.String("cni", "canal", "calico, canal, cilium, or flannel")

func Test_E2ENightlyCNI(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Validate dualstack in "+*cni+", "+*filter+" Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig
var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify DualStack in "+*cni+", "+*filter+" configuration", Ordered, func() {
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

	It("Checks rke2 configuration", func() {
		for _, server := range tc.Servers {
			cmd := "cat /etc/rancher/rke2/config.yaml"
			res, err := server.RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
			fmt.Printf("config.yaml for %s:\n%s\n", server.Name, res)
			Expect(res).Should(ContainSubstring("cni: " + *cni))
			Expect(res).Should(ContainSubstring("proxy-mode=" + *filter))
		}
	})

	It("Checks Node Status", func() {
		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"))
			}
		}, "720s", "5s").Should(Succeed())
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
		pods, err := e2e.ParsePods(tc.KubeconfigFile, true)
		fmt.Printf("Pods: %v\n", pods)
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

	It("Waits for CNI daemonset readiness", func() {
		type cniInfo struct {
			dsName    string
			namespace string
		}
		cniDaemonsets := map[string]cniInfo{
			"calico":  {"calico-node", "calico-system"},
			"canal":   {"rke2-canal", "kube-system"},
			"cilium":  {"cilium", "kube-system"},
			"flannel": {"kube-flannel-ds", "kube-system"},
		}
		info, ok := cniDaemonsets[*cni]
		if !ok {
			Skip("unknown CNI: " + *cni)
		}
		Eventually(func() error {
			_, err := e2e.RunCommand("kubectl -n " + info.namespace + " rollout status ds/" + info.dsName + " --timeout=120s --kubeconfig=" + tc.KubeconfigFile)
			return err
		}, "180s", "5s").Should(Succeed())
	})

	It("Waits for traefik daemonset readiness", func() {
		By("waiting for traefik daemonset readiness")
		Eventually(func() error {
			_, err := e2e.RunCommand("kubectl -n kube-system rollout status ds/rke2-traefik --timeout=120s --kubeconfig=" + tc.KubeconfigFile)
			return err
		}, "180s", "5s").Should(Succeed())
	})

	It("Checks filtering backend", func() {
		for _, node := range tc.AllNodes() {
			var cmd string
			switch *cni {
			case "calico":
				if *filter == "nftables" {
					cmd = "nft list ruleset | grep 'cali-'"
				} else {
					cmd = "iptables -L | grep 'cali-'"
				}
			case "flannel":
				if *filter == "nftables" {
					cmd = "nft list ruleset | grep 'flannel'"
				} else {
					cmd = "iptables -L | grep 'FLANNEL'"
				}
			case "canal":
				var calicoCmd, flannelCmd string
				if *filter == "nftables" {
					calicoCmd = "nft list ruleset | grep 'cali-'"
					flannelCmd = "nft list ruleset | grep 'flannel'"
				} else {
					calicoCmd = "iptables -L | grep 'cali-'"
					flannelCmd = "iptables -L -t nat| grep 'FLANNEL'"
				}
				res, err := node.RunCmdOnNode(calicoCmd)
				Expect(err).NotTo(HaveOccurred(), "canal calico filtering backend check failed on %s: %s", node.Name, res)
				res, err = node.RunCmdOnNode(flannelCmd)
				Expect(err).NotTo(HaveOccurred(), "canal flannel filtering backend check failed on %s: %s", node.Name, res)
				continue
			case "cilium":
				continue
			default:
				continue
			}
			res, err := node.RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "filtering backend check failed on %s: %s", node.Name, res)
		}
	})

	It("Verifies ClusterIP Service", func() {
		_, err := tc.DeployWorkload("dualstack_clusterip.yaml")
		Expect(err).NotTo(HaveOccurred())
		Eventually(e2e.RunCommand, "120s", "5s").WithArguments("kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile).Should(ContainSubstring("ds-clusterip-pod"))

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
				Eventually(tc.Servers[0].RunCmdOnNode, "120s", "5s").WithArguments(cmd).Should(ContainSubstring("Welcome to nginx!"), "failed cmd: "+cmd)
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
			Eventually(e2e.RunCommand, "60s", "5s").WithArguments(cmd).Should(ContainSubstring("client-deployment"), "failed cmd: "+cmd)
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
			Eventually(e2e.RunCommand, "60s", "5s").WithArguments(cmd).Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
			cmd = fmt.Sprintf("curl  --header host:%s http://[%s]/name.html", hostName, node.Ipv6)
			Eventually(e2e.RunCommand, "60s", "5s").WithArguments(cmd).Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
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
			Eventually(e2e.RunCommand, "60s", "5s").WithArguments(cmd).Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
			cmd = "curl -L --insecure http://[" + node.Ipv6 + "]:" + nodeport + "/name.html"
			Eventually(e2e.RunCommand, "60s", "5s").WithArguments(cmd).Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
		}
	})
})

var failed bool
var _ = AfterEach(func() {

	failed = failed || CurrentSpecReport().Failed()
	if CurrentSpecReport().Failed() {
		type cniInfo struct {
			dsName    string
			namespace string
		}
		cniDaemonsets := map[string]cniInfo{
			"calico":  {"calico-node", "calico-system"},
			"canal":   {"rke2-canal", "kube-system"},
			"cilium":  {"cilium", "kube-system"},
			"flannel": {"kube-flannel-ds", "kube-system"},
		}
		info, ok := cniDaemonsets[*cni]
		if !ok {
			Skip("unknown CNI: " + *cni)
		}
		GinkgoWriter.Println("=== CNI (" + info.dsName + ") pod logs ===")
		out, _ := e2e.RunCommand("kubectl -n " + info.namespace + " logs ds/" + info.dsName + " --tail=100 --kubeconfig=" + tc.KubeconfigFile)
		GinkgoWriter.Println(out)

		GinkgoWriter.Println("=== kubectl get events ===")
		out, _ = e2e.RunCommand("kubectl get events -A --sort-by='.lastTimestamp' --kubeconfig=" + tc.KubeconfigFile)
		GinkgoWriter.Println(out)
	}
})

var _ = AfterSuite(func() {
	if failed && !*ci {
		fmt.Println("FAILED!")
	} else {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(tc.KubeconfigFile)).To(Succeed())
	}
})
