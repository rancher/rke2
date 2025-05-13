package splitserver

import (
	"flag"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests"
	"github.com/rancher/rke2/tests/docker"
)

var (
	etcdCount         = flag.Int("etcdCount", 3, "number of server nodes only deploying etcd")
	controlPlaneCount = flag.Int("controlPlaneCount", 1, "number of server nodes acting as control plane")
	agentCount        = flag.Int("agentCount", 2, "number of agent nodes")
	ci                = flag.Bool("ci", false, "running on CI")

	tc        *docker.TestConfig
	cpNodes   []docker.DockerNode
	etcdNodes []docker.DockerNode
)

func Test_DockerSplitServer(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "Split Server Test Suite")
}

var _ = Describe("Verify Create", Ordered, func() {
	Context("Setup Cluster", func() {
		It("should provision servers and agents", func() {
			var err error
			tc, err = docker.NewTestConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(tc.ProvisionServers(*etcdCount + *controlPlaneCount)).To(Succeed())
			Expect(tc.ProvisionAgents(*agentCount)).To(Succeed())
			etcdNodes = tc.Servers[:*etcdCount]
			cpNodes = tc.Servers[*etcdCount:]
			Expect(setupEtcd(etcdNodes)).To(Succeed())
			Expect(setupCP(cpNodes)).To(Succeed())
			Expect(docker.RestartCluster(append(tc.Servers, tc.Agents...))).To(Succeed())
			Expect(tc.CopyAndModifyKubeconfig()).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(tests.CheckDefaultDeployments(tc.KubeconfigFile)).To(Succeed())
				g.Expect(tests.CheckDefaultDaemonSets(tc.KubeconfigFile)).To(Succeed())
			}, "240s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})
	Context("Validate various components", func() {
		It("Verifies ClusterIP Service", func() {
			_, err := tc.DeployWorkload("clusterip.yaml")
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)

			clusterip, _ := docker.FetchClusterIP(tc.KubeconfigFile, "nginx-clusterip-svc")
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
				cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + tc.KubeconfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
				nodeport, err := docker.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())

				cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
				Eventually(func() (string, error) {
					return docker.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")

				cmd = "curl -L --insecure http://" + cp.IP + ":" + nodeport + "/name.html"
				Eventually(func() (string, error) {
					return docker.RunCommand(cmd)
				}, "240s", "5s").Should(ContainSubstring("test-nodeport"), "failed cmd: "+cmd)
			}
		})

		It("Verifies dns access", func() {
			_, err := tc.DeployWorkload("dnsutils.yaml")
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")
			Eventually(func() (bool, error) {
				return tests.PodReady("dnsutils", "default", tc.KubeconfigFile)
			}, "20s", "5s").Should(BeTrue())

			cmd := "kubectl --kubeconfig=" + tc.KubeconfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "120s", "2s").Should(ContainSubstring("kubernetes.default.svc.cluster.local"), "failed cmd: "+cmd)
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if tc != nil && failed {
		AddReportEntry("cluster-resources", tc.DumpResources())
		AddReportEntry("pod-logs", tc.DumpPodLogs(50))
		AddReportEntry("journald-logs", tc.DumpServiceLogs(250))
		AddReportEntry("component-logs", tc.DumpComponentLogs(250))
	}
	if *ci || (tc != nil && !failed) {
		tc.Cleanup()
	}
})

func setupEtcd(etcdNodes []docker.DockerNode) error {
	etcdYAML := `
disable-apiserver: true
disable-controller-manager: true
disable-scheduler: true
node-taint:
- node-role.kubernetes.io/etcd:NoExecute
`
	for _, node := range etcdNodes {
		cmd := fmt.Sprintf("echo '%s' >> /etc/rancher/rke2/config.yaml", etcdYAML)
		if out, err := node.RunCmdOnNode(cmd); err != nil {
			return fmt.Errorf("failed to write etcd yaml: %s: %v", out, err)
		}
	}
	return nil
}

func setupCP(cpNodes []docker.DockerNode) error {
	cpYAML := `
disable-etcd: true
node-taint:
- node-role.kubernetes.io/control-plane:NoSchedule
`
	for _, node := range cpNodes {
		cmd := fmt.Sprintf("echo '%s' >> /etc/rancher/rke2/config.yaml", cpYAML)
		if out, err := node.RunCmdOnNode(cmd); err != nil {
			return fmt.Errorf("failed to write etcd yaml: %s: %v", out, err)
		}
	}
	return nil
}
