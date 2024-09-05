package ciliumnokp

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

func Test_E2ECiliumNoKP(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Validate dualstack in Cilium without kube-proxy Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)
var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify DualStack in Cilium without kube-proxy configuration", Ordered, func() {

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

	It("Verifies that cilium config is correct", func() {
		cmdCiliumAgents := "kubectl get pods -l app.kubernetes.io/name=cilium-agent -n kube-system -o=name --kubeconfig=" + kubeConfigFile
		res, err := e2e.RunCommand(cmdCiliumAgents)
		Expect(err).NotTo(HaveOccurred(), res)
		ciliumAgents := strings.Split(strings.TrimSpace(res), "\n")
		Expect(len(ciliumAgents)).Should(Equal(len(serverNodeNames) + len(agentNodeNames)))
		for _, ciliumAgent := range ciliumAgents {
			cmd := "kubectl exec " + ciliumAgent + " -n kube-system  -c cilium-agent --kubeconfig=" + kubeConfigFile + " -- cilium-dbg status --verbose | grep -e 'BPF' -e 'HostPort' -e 'LoadBalancer'"
			res, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
			// We expect the following output and the important parts are HostPort, LoadBalancer, Host Routing and Masquerading
			// Routing:                Network: Tunnel [vxlan]   Host: BPF
			// Masquerading:           BPF
			// Clock Source for BPF:   ktime
			// - LoadBalancer:   Enabled
			// - HostPort:       Enabled
			// BPF Maps:   dynamic sizing: on (ratio: 0.002500)
			Expect(res).Should(ContainSubstring("Routing"))
			Expect(res).Should(ContainSubstring("Masquerading"))
			Expect(res).Should(ContainSubstring("LoadBalancer:   Enabled"))
			Expect(res).Should(ContainSubstring("HostPort:       Enabled"))
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

	It("Verifies internode connectivity", func() {
		_, err := e2e.DeployWorkload("pod_client.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())

		// Wait for the pod_client to have an IP
		Eventually(func() string {
			ips, _ := e2e.PodIPsUsingLabel(kubeConfigFile, "app=client")
			return ips[0].Ipv4
		}, "40s", "5s").Should(ContainSubstring("10.42"), "failed getClientIPs")

		clientIPs, err := e2e.PodIPsUsingLabel(kubeConfigFile, "app=client")
		Expect(err).NotTo(HaveOccurred())
		for _, ip := range clientIPs {
			cmd := "kubectl exec svc/client-curl --kubeconfig=" + kubeConfigFile + " -- curl -m7 " + ip.Ipv4 + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "20s", "3s").Should(ContainSubstring("client-deployment"), "failed cmd: "+cmd)
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
			}, "30s", "2s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
			cmd = fmt.Sprintf("curl  --header host:%s http://[%s]/name.html", hostName, node.Ipv6)
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "10s", "1s").Should(ContainSubstring("ds-clusterip-pod"), "failed cmd: "+cmd)
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
			}, "30s", "1s").Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
			cmd = "curl -L --insecure http://[" + node.Ipv6 + "]:" + nodeport + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "10s", "1s").Should(ContainSubstring("ds-nodeport-pod"), "failed cmd: "+cmd)
		}
	})

	It("Verifies there are no required iptables", func() {
		// Check that there are no iptables rules with KUBE-SVC and HOSTPORT
		cmdiptables := "sudo iptables-save | grep -e 'KUBE-SVC' -e 'HOSTPORT' | wc -l"
		for i := range serverNodeNames {
			res, err := e2e.RunCmdOnNode(cmdiptables, serverNodeNames[i])
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
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
