package clusterloadbalancer

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/e2e"
)

// This suite reproduces the external load-balancer setup documented at
// https://docs.rke2.io/networking/cluster-loadbalancer : an HA embedded-etcd cluster
// fronted by an HAProxy + Keepalived virtual IP (VIP) used as the fixed registration
// address (9345) and API server endpoint (6443). Servers beyond the first, and all
// agents, join the cluster through the VIP rather than a concrete server IP.

// Valid nodeOS: bento/ubuntu-24.04
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built RKE2")
var cni = flag.String("cni", "canal", "canal or calico")
var dataplane = flag.String("dataplane", "iptables", "iptables or ebpf")

// The virtual IP fronted by the load balancer. Must match the Vagrantfile.
const vip = "10.10.10.100"

func Test_E2EClusterLoadBalancer(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Cluster Load Balancer ("+*cni+", "+*dataplane+") Test Suite", suiteConfig, reporterConfig)
}

// createLBCluster brings the nodes up in a deterministic order: the load balancer first
// so the VIP exists, then the cluster-init server, then the remaining servers and agents
// which register through the VIP.
func createLBCluster(nodeOS string, serverCount, agentCount int) (e2e.VagrantNode, []e2e.VagrantNode, []e2e.VagrantNode, error) {
	lbNode := e2e.VagrantNode{Name: "lb-0", Type: e2e.Linux}
	serverNodes := make([]e2e.VagrantNode, serverCount)
	for i := 0; i < serverCount; i++ {
		serverNodes[i] = e2e.VagrantNode{Name: "server-" + strconv.Itoa(i), Type: e2e.Linux}
	}
	agentNodes := make([]e2e.VagrantNode, agentCount)
	for i := 0; i < agentCount; i++ {
		agentNodes[i] = e2e.VagrantNode{Name: "agent-" + strconv.Itoa(i), Type: e2e.Linux}
	}

	allNodes := append([]e2e.VagrantNode{lbNode}, serverNodes...)
	allNodes = append(allNodes, agentNodes...)
	nodeRoles := strings.Join(e2e.VagrantSlice(allNodes), " ")
	nodeBoxes := strings.TrimSpace(strings.Repeat(nodeOS+" ", len(allNodes)))

	var testOptions string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}
	nodeEnvs := fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s"`, nodeRoles, nodeBoxes)

	if *local {
		testOptions += " E2E_RELEASE_VERSION=skip"
		// Bring all nodes up without provisioning first so the VM images are imported.
		allNodesStr := strings.Join(e2e.VagrantSlice(allNodes), " ")
		cmd := fmt.Sprintf(`%s %s E2E_STANDUP_PARALLEL=true vagrant up --no-tty --no-provision %s &> vagrant.log`, nodeEnvs, testOptions, allNodesStr)
		fmt.Println(cmd)
		if _, err := e2e.RunCommand(cmd); err != nil {
			return lbNode, serverNodes, agentNodes, fmt.Errorf("failed to bring up nodes: %w", err)
		}
		// SCP locally built artifacts to every RKE2 node (lb node runs HAProxy, not RKE2).
		if err := e2e.SCPRke2Artifacts(append(serverNodes, agentNodes...)); err != nil {
			return lbNode, serverNodes, agentNodes, err
		}
		// Provision nodes in the required order: lb, server-0, then the rest.
		for _, node := range append([]e2e.VagrantNode{lbNode, serverNodes[0]}, append(serverNodes[1:], agentNodes...)...) {
			cmd := fmt.Sprintf(`%s %s vagrant provision --no-tty %s &>> vagrant.log`, nodeEnvs, testOptions, node.Name)
			fmt.Println(cmd)
			if _, err := e2e.RunCommand(cmd); err != nil {
				return lbNode, serverNodes, agentNodes, fmt.Errorf("failed to provision %s: %w", node.Name, err)
			}
		}
		return lbNode, serverNodes, agentNodes, nil
	}

	// Bring up the load balancer (VIP) and then the cluster-init server sequentially.
	for _, node := range []e2e.VagrantNode{lbNode, serverNodes[0]} {
		cmd := fmt.Sprintf(`%s %s vagrant up --no-tty %s &>> vagrant.log`, nodeEnvs, testOptions, node.Name)
		fmt.Println(cmd)
		if _, err := e2e.RunCommand(cmd); err != nil {
			return lbNode, serverNodes, agentNodes, fmt.Errorf("failed to bring up %s: %w", node.Name, err)
		}
	}

	// Bring up the remaining servers and agents, which register through the VIP.
	for _, node := range append(serverNodes[1:], agentNodes...) {
		cmd := fmt.Sprintf(`%s %s vagrant up --no-tty %s &>> vagrant.log`, nodeEnvs, testOptions, node.Name)
		fmt.Println(cmd)
		if _, err := e2e.RunCommand(cmd); err != nil {
			return lbNode, serverNodes, agentNodes, fmt.Errorf("failed to bring up %s: %w", node.Name, err)
		}
	}

	return lbNode, serverNodes, agentNodes, nil
}

// genVIPKubeConfigFile writes a kubeconfig whose server address is the load balancer VIP
// on the API server port, proving that HAProxy fronts the Kubernetes API.
func genVIPKubeConfigFile(server e2e.VagrantNode) (string, error) {
	kubeConfig, err := server.RunCmdOnNode("cat /etc/rancher/rke2/rke2.yaml")
	if err != nil {
		return "", err
	}
	kubeConfig = strings.Replace(kubeConfig, "127.0.0.1", vip, 1)
	kubeConfigFile := "kubeconfig-vip"
	if err := os.WriteFile(kubeConfigFile, []byte(kubeConfig), 0644); err != nil {
		return "", err
	}
	return kubeConfigFile, nil
}

var (
	lbNode      e2e.VagrantNode
	serverNodes []e2e.VagrantNode
	agentNodes  []e2e.VagrantNode
	tc          *e2e.TestConfig
)

var _ = ReportAfterEach(e2e.GenReport)

func dumpCommand(title, cmd string) {
	GinkgoWriter.Println("=== " + title + " ===")
	out, err := e2e.RunCommand(cmd)
	if err != nil {
		GinkgoWriter.Println("error:", err)
	}
	GinkgoWriter.Println(out)
}

func dumpClusterDiagnostics() {
	if tc == nil || tc.KubeconfigFile == "" {
		return
	}

	kubeconfig := " --kubeconfig=" + tc.KubeconfigFile
	dumpCommand("kubectl describe nodes", "kubectl describe nodes"+kubeconfig)
	dumpCommand("kubectl get pods -A -o wide", "kubectl get pods -A -o wide"+kubeconfig)
	dumpCommand("kubectl describe non-running pods",
		"kubectl get pods -A --no-headers"+kubeconfig+" | awk '$4 != \"Running\" && $4 != \"Completed\" {print $1, $2}' | while read -r ns pod; do echo \"--- ${ns}/${pod}\"; kubectl -n \"$ns\" describe pod \"$pod\""+kubeconfig+"; done")
	dumpCommand("kubectl get events", "kubectl get events -A --sort-by='.lastTimestamp'"+kubeconfig)
	dumpCommand("calico-kube-controllers logs",
		"kubectl get pods -A --no-headers"+kubeconfig+" | awk '$2 ~ /calico-kube-controllers/ {print $1, $2}' | while read -r ns pod; do echo \"--- ${ns}/${pod}\"; kubectl -n \"$ns\" logs \"$pod\" --all-containers --tail=100"+kubeconfig+"; done")
	dumpCommand("calico-node logs",
		"kubectl get pods -A --no-headers"+kubeconfig+" | awk '$2 ~ /calico-node/ {print $1, $2}' | while read -r ns pod; do echo \"--- ${ns}/${pod}\"; kubectl -n \"$ns\" logs \"$pod\" --all-containers --tail=100"+kubeconfig+"; done")
}

func dumpLoadBalancerDiagnostics() {
	if lbNode.Name == "" {
		return
	}

	GinkgoWriter.Println("=== load balancer diagnostics ===")
	out, err := lbNode.RunCmdOnNode("systemctl status haproxy keepalived --no-pager; echo '--- addresses'; ip addr show eth1; echo '--- listeners'; ss -ltnp | grep -E ':(6443|9345)'; echo '--- haproxy journal'; journalctl -u haproxy --no-pager -n 100")
	if err != nil {
		GinkgoWriter.Println("error:", err)
	}
	GinkgoWriter.Println(out)
}

var _ = Describe("Verify external load balancer cluster", Ordered, func() {
	It("Starts up with no issues", func() {
		var err error
		lbNode, serverNodes, agentNodes, err = createLBCluster(*nodeOS, *serverCount, *agentCount)
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
		tc = &e2e.TestConfig{
			Servers: serverNodes,
			Agents:  agentNodes,
		}
		By("CLUSTER CONFIG")
		By("OS: " + *nodeOS)
		By("Load balancer: " + lbNode.Name + " (VIP " + vip + ")")
		By(tc.Status())
		tc.KubeconfigFile, err = e2e.GenKubeConfigFile(serverNodes[0])
		Expect(err).NotTo(HaveOccurred())
	})

	It("Configures the load balancer node with HAProxy and Keepalived", func() {
		Eventually(func(g Gomega) {
			res, err := lbNode.RunCmdOnNode("systemctl is-active haproxy")
			g.Expect(err).NotTo(HaveOccurred(), res)
			g.Expect(res).Should(ContainSubstring("active"))
			res, err = lbNode.RunCmdOnNode("systemctl is-active keepalived")
			g.Expect(err).NotTo(HaveOccurred(), res)
			g.Expect(res).Should(ContainSubstring("active"))
		}, "60s", "5s").Should(Succeed())

		By("Verifying the VIP is assigned to the load balancer node")
		Eventually(func(g Gomega) {
			res, err := lbNode.RunCmdOnNode("ip addr show " + "eth1")
			g.Expect(err).NotTo(HaveOccurred(), res)
			g.Expect(res).Should(ContainSubstring(vip))
		}, "60s", "5s").Should(Succeed())
	})

	It("Registers joining nodes through the VIP", func() {
		// The cluster-init server (server-0) must not point at the VIP.
		res, err := serverNodes[0].RunCmdOnNode("cat /etc/rancher/rke2/config.yaml")
		Expect(err).NotTo(HaveOccurred(), res)
		Expect(res).ShouldNot(ContainSubstring("server: https://" + vip))
		Expect(res).Should(ContainSubstring("tls-san"))

		// Every other server and agent must join via the VIP.
		joiners := append(serverNodes[1:], agentNodes...)
		for _, node := range joiners {
			res, err := node.RunCmdOnNode("cat /etc/rancher/rke2/config.yaml")
			Expect(err).NotTo(HaveOccurred(), res)
			Expect(res).Should(ContainSubstring("server: https://"+vip+":9345"), node.Name)
		}
	})

	It("Checks Node Status", func() {
		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(nodes).Should(HaveLen(*serverCount + *agentCount))
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"), node.Name)
			}
		}, "720s", "10s").Should(Succeed())
		_, err := e2e.ParseNodes(tc.KubeconfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Reaches the API server through the load balancer VIP", func() {
		vipKubeConfig, err := genVIPKubeConfigFile(serverNodes[0])
		Expect(err).NotTo(HaveOccurred())
		defer os.Remove(vipKubeConfig)

		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(vipKubeConfig, false)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(nodes).Should(HaveLen(*serverCount + *agentCount))
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"), node.Name)
			}
		}, "120s", "5s").Should(Succeed())
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
		}, "620s", "10s").Should(Succeed())
		_, err := e2e.ParsePods(tc.KubeconfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Verifies the eBPF dataplane disables kube-proxy", func() {
		if *dataplane != "ebpf" {
			Skip("not an eBPF dataplane run")
		}
		// The calico HelmChartConfig must point at the VIP so pods reach the API server
		// with kube-proxy disabled.
		res, err := serverNodes[0].RunCmdOnNode("cat /var/lib/rancher/rke2/server/manifests/rke2-calico-config.yaml")
		Expect(err).NotTo(HaveOccurred(), res)
		Expect(res).Should(ContainSubstring("host: \"" + vip + "\""))
		Expect(res).Should(ContainSubstring("port: \"6443\""))
		Expect(res).ShouldNot(ContainSubstring("nodeAddressAutodetectionV6"))

		// With kube-proxy disabled, no KUBE-SVC iptables rules should exist.
		for _, server := range serverNodes {
			res, err := server.RunCmdOnNode("iptables-save | grep -e 'KUBE-SVC' | wc -l")
			Expect(err).NotTo(HaveOccurred(), res)
			Expect(strings.TrimSpace(res)).Should(Equal("0"), server.Name)
		}
	})

	It("Verifies ClusterIP Service", func() {
		_, err := tc.DeployWorkload("clusterip.yaml")
		Expect(err).NotTo(HaveOccurred(), "ClusterIP manifest not deployed")

		cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
		Eventually(func() (string, error) {
			return e2e.RunCommand(cmd)
		}, "240s", "5s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)

		clusterip, _ := e2e.FetchClusterIP(tc.KubeconfigFile, "nginx-clusterip-svc", false)
		cmd = "curl -L --insecure http://" + clusterip + "/name.html"
		for _, server := range serverNodes {
			Eventually(func() (string, error) {
				return server.RunCmdOnNode(cmd)
			}, "120s", "10s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)
		}
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
	if CurrentSpecReport().Failed() {
		dumpClusterDiagnostics()
		dumpLoadBalancerDiagnostics()
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
