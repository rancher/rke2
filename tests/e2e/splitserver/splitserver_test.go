package splitserver

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

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var etcdCount = flag.Int("etcdCount", 1, "number of server nodes only deploying etcd")
var controlPlaneCount = flag.Int("controlPlaneCount", 1, "number of server nodes acting as control plane")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+rke2r1 or nil for latest commit from master

func createSplitCluster(nodeOS string, etcdCount, controlPlaneCount, agentCount int) ([]e2e.VagrantNode, []e2e.VagrantNode, []e2e.VagrantNode, error) {
	etcdNodes := make([]e2e.VagrantNode, etcdCount)
	for i := 0; i < etcdCount; i++ {
		etcdNodes[i] = e2e.VagrantNode{"server-etcd-" + strconv.Itoa(i), e2e.Linux}
	}
	cpNodes := make([]e2e.VagrantNode, controlPlaneCount)
	for i := 0; i < controlPlaneCount; i++ {
		cpNodes[i] = e2e.VagrantNode{"server-cp-" + strconv.Itoa(i), e2e.Linux}
	}
	agentNodes := make([]e2e.VagrantNode, agentCount)
	for i := 0; i < agentCount; i++ {
		agentNodes[i] = e2e.VagrantNode{"agent-" + strconv.Itoa(i), e2e.Linux}
	}
	nodeRoles := strings.Join(e2e.VagrantSlice(etcdNodes), " ") + " " + strings.Join(e2e.VagrantSlice(cpNodes), " ") + " " + strings.Join(e2e.VagrantSlice(agentNodes), " ")

	nodeRoles = strings.TrimSpace(nodeRoles)
	nodeBoxes := strings.Repeat(nodeOS+" ", etcdCount+controlPlaneCount+agentCount)
	nodeBoxes = strings.TrimSpace(nodeBoxes)

	var testOptions string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}

	cmd := fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s" %s vagrant up --no-tty &> vagrant.log`, nodeRoles, nodeBoxes, testOptions)
	fmt.Println(cmd)
	if _, err := e2e.RunCommand(cmd); err != nil {
		fmt.Println("Error Creating Cluster", err)
		return nil, nil, nil, err
	}
	return etcdNodes, cpNodes, agentNodes, nil
}
func Test_E2ESplitServer(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Split Server Test Suite", suiteConfig, reporterConfig)
}

var (
	etcdNodes  []e2e.VagrantNode
	cpNodes    []e2e.VagrantNode
	agentNodes []e2e.VagrantNode
	tc         e2e.TestConfig // Only used to store the kubeconfigfile
)
var _ = ReportAfterEach(e2e.GenReport)
var _ = Describe("Verify Create", Ordered, func() {
	Context("Cluster :", func() {
		It("Starts up with no issues", func() {
			tc = e2e.TestConfig{}
			var err error
			etcdNodes, cpNodes, agentNodes, err = createSplitCluster(*nodeOS, *etcdCount, *controlPlaneCount, *agentCount)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", *nodeOS)
			fmt.Println("Etcd Server Nodes:", etcdNodes)
			fmt.Println("Control Plane Server Nodes:", cpNodes)
			fmt.Println("Agent Nodes:", agentNodes)
			tc.KubeconfigFile, err = e2e.GenKubeConfigFile(cpNodes[0])
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks Node and Pod Status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(tc.KubeconfigFile, true)

			fmt.Printf("\nFetching Pods status\n")
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
			_, _ = e2e.ParsePods(tc.KubeconfigFile, true)
		})

		It("Verifies ClusterIP Service", func() {
			_, err := tc.DeployWorkload("clusterip.yaml")
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)

			clusterip, _ := e2e.FetchClusterIP(tc.KubeconfigFile, "nginx-clusterip-svc", false)
			cmd = "curl -L --insecure http://" + clusterip + "/name.html"
			for _, cp := range cpNodes {
				Eventually(func() (string, error) {
					return cp.RunCmdOnNode(cmd)
				}, "120s", "10s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)
			}
		})

		It("Verifies NodePort Service", func() {
			_, err := tc.DeployWorkload("nodeport.yaml")
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")

			for _, cp := range cpNodes {
				nodeExternalIP, _ := cp.FetchNodeExternalIP()
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := e2e.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")

				cmd = "curl -L --insecure http://" + nodeExternalIP + ":" + nodeport + "/name.html"
				Eventually(func() (string, error) {
					return e2e.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "failed cmd: "+cmd)
			}
		})

		It("Verifies LoadBalancer Service", func() {
			_, err := tc.DeployWorkload("loadbalancer.yaml")
			Expect(err).NotTo(HaveOccurred(), "Loadbalancer manifest not deployed")

			cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.externalIPs[0]}:{.spec.ports[0].port}\""
			ipPort, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"))

			cmd = "curl -L --insecure http://" + ipPort + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"), "failed cmd: "+cmd)
		})

		It("Verifies Daemonset", func() {
			_, err := tc.DeployWorkload("daemonset.yaml")
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			Eventually(func(g Gomega) {
				pods, _ := e2e.ParsePods(tc.KubeconfigFile, false)
				count := e2e.CountOfStringInSlice("test-daemonset", pods)
				fmt.Println("POD COUNT")
				fmt.Println(count)
				fmt.Println("CP COUNT")
				fmt.Println(len(cpNodes))
				g.Expect(len(cpNodes)).Should((Equal(count)), "Daemonset pod count does not match cp node count")
			}, "240s", "10s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			_, err := tc.DeployWorkload("dnsutils.yaml")
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			cmd := "kubectl get pods dnsutils --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("dnsutils"), "failed cmd: "+cmd)

			cmd = "kubectl --kubeconfig=" + tc.KubeconfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "420s", "2s").Should(ContainSubstring("kubernetes.default.svc.cluster.local"), "failed cmd: "+cmd)
		})
	})
})

var failed = false
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
