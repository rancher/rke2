package kubevip

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

// This suite is the kube-vip counterpart to the clusterloadbalancer suite. It reproduces the
// external load-balancer setup documented at https://docs.rke2.io/networking/cluster-loadbalancer,
// but instead of an HAProxy + Keepalived VIP running on a dedicated node, the virtual IP (VIP) is
// provided by kube-vip running as a control-plane DaemonSet on the server nodes. kube-vip needs no
// dedicated VM: in ARP mode the VIP floats onto whichever server node holds the control-plane
// lease, and that node already serves the registration address (9345) and API server (6443)
// locally. Servers beyond the first, and all agents, join the cluster through the VIP.

// Valid nodeOS: bento/ubuntu-24.04
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var cni = flag.String("cni", "canal", "canal or calico")
var dataplane = flag.String("dataplane", "iptables", "iptables or ebpf")

// The virtual IP fronted by kube-vip. Must match the Vagrantfile.
const vip = "10.10.10.100"

// The interface kube-vip advertises the VIP on. Must match the Vagrantfile.
const vipInterface = "eth1"

func Test_E2EKubeVIP(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Cluster Load Balancer kube-vip ("+*cni+", "+*dataplane+") Test Suite", suiteConfig, reporterConfig)
}

// createKubeVIPCluster brings the nodes up in a deterministic order: the cluster-init server first,
// so it can bring the cluster up and let kube-vip claim the VIP, then the remaining servers and
// agents which register through the VIP.
func createKubeVIPCluster(nodeOS string, serverCount, agentCount int) ([]e2e.VagrantNode, []e2e.VagrantNode, error) {
	serverNodes := make([]e2e.VagrantNode, serverCount)
	for i := 0; i < serverCount; i++ {
		serverNodes[i] = e2e.VagrantNode{Name: "server-" + strconv.Itoa(i), Type: e2e.Linux}
	}
	agentNodes := make([]e2e.VagrantNode, agentCount)
	for i := 0; i < agentCount; i++ {
		agentNodes[i] = e2e.VagrantNode{Name: "agent-" + strconv.Itoa(i), Type: e2e.Linux}
	}

	allNodes := append([]e2e.VagrantNode{}, serverNodes...)
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

	// Bring up the cluster-init server first so the cluster is up and kube-vip can claim the VIP.
	cmd := fmt.Sprintf(`%s %s vagrant up --no-tty %s &>> vagrant.log`, nodeEnvs, testOptions, serverNodes[0].Name)
	fmt.Println(cmd)
	if _, err := e2e.RunCommand(cmd); err != nil {
		return serverNodes, agentNodes, fmt.Errorf("failed to bring up %s: %w", serverNodes[0].Name, err)
	}

	// Bring up the remaining servers and agents, which register through the VIP.
	for _, node := range append(serverNodes[1:], agentNodes...) {
		cmd := fmt.Sprintf(`%s %s vagrant up --no-tty %s &>> vagrant.log`, nodeEnvs, testOptions, node.Name)
		fmt.Println(cmd)
		if _, err := e2e.RunCommand(cmd); err != nil {
			return serverNodes, agentNodes, fmt.Errorf("failed to bring up %s: %w", node.Name, err)
		}
	}

	return serverNodes, agentNodes, nil
}

// genVIPKubeConfigFile writes a kubeconfig whose server address is the kube-vip VIP on the API
// server port, proving that kube-vip fronts the Kubernetes API.
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
	dumpCommand("kube-vip logs",
		"kubectl get pods -n kube-system --no-headers"+kubeconfig+" | awk '$1 ~ /kube-vip-ds/ {print $1}' | while read -r pod; do echo \"--- kube-system/${pod}\"; kubectl -n kube-system logs \"$pod\" --all-containers --tail=100"+kubeconfig+"; done")
	dumpCommand("calico-kube-controllers logs",
		"kubectl get pods -A --no-headers"+kubeconfig+" | awk '$2 ~ /calico-kube-controllers/ {print $1, $2}' | while read -r ns pod; do echo \"--- ${ns}/${pod}\"; kubectl -n \"$ns\" logs \"$pod\" --all-containers --tail=100"+kubeconfig+"; done")
	dumpCommand("calico-node logs",
		"kubectl get pods -A --no-headers"+kubeconfig+" | awk '$2 ~ /calico-node/ {print $1, $2}' | while read -r ns pod; do echo \"--- ${ns}/${pod}\"; kubectl -n \"$ns\" logs \"$pod\" --all-containers --tail=100"+kubeconfig+"; done")
}

// dumpVIPDiagnostics reports which server node currently holds the VIP and the kube-vip lease.
func dumpVIPDiagnostics() {
	GinkgoWriter.Println("=== kube-vip diagnostics ===")
	for _, server := range serverNodes {
		out, err := server.RunCmdOnNode("ip addr show " + vipInterface + " | grep -F " + vip + " || echo 'VIP not present'")
		if err != nil {
			GinkgoWriter.Println(server.Name, "error:", err)
			continue
		}
		GinkgoWriter.Println("--- "+server.Name+":", strings.TrimSpace(out))
	}
}

var _ = Describe("Verify kube-vip load balancer cluster", Ordered, func() {
	It("Starts up with no issues", func() {
		var err error
		serverNodes, agentNodes, err = createKubeVIPCluster(*nodeOS, *serverCount, *agentCount)
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
		tc = &e2e.TestConfig{
			Servers: serverNodes,
			Agents:  agentNodes,
		}
		By("CLUSTER CONFIG")
		By("OS: " + *nodeOS)
		By("Load balancer: kube-vip (VIP " + vip + ")")
		By(tc.Status())
		tc.KubeconfigFile, err = e2e.GenKubeConfigFile(serverNodes[0])
		Expect(err).NotTo(HaveOccurred())
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

	It("Runs the kube-vip control-plane DaemonSet on the server nodes", func() {
		Eventually(func(g Gomega) {
			pods, err := e2e.ParsePods(tc.KubeconfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			var kubeVIPPods int
			for _, pod := range pods {
				if strings.HasPrefix(pod.Name, "kube-vip-ds") {
					kubeVIPPods++
					g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
				}
			}
			// One kube-vip pod per control-plane (server) node.
			g.Expect(kubeVIPPods).Should(Equal(*serverCount), "expected one kube-vip pod per server node")
		}, "300s", "10s").Should(Succeed())
	})

	It("Assigns the VIP to a server node", func() {
		Eventually(func(g Gomega) {
			var holders int
			for _, server := range serverNodes {
				res, err := server.RunCmdOnNode("ip addr show " + vipInterface)
				g.Expect(err).NotTo(HaveOccurred(), res)
				if strings.Contains(res, vip) {
					holders++
				}
			}
			// Exactly one server node holds the VIP at any time (leader election).
			g.Expect(holders).Should(Equal(1), "expected exactly one server node to hold the VIP")
		}, "120s", "5s").Should(Succeed())
	})

	It("Reaches the API server through the VIP", func() {
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
		dumpVIPDiagnostics()
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
