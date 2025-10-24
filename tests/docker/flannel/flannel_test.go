package flannel

import (
	"flag"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests"
	"github.com/rancher/rke2/tests/docker"
)

type pod struct {
	ip   string
	name string
}

var (
	serverCount = flag.Int("serverCount", 1, "number of server nodes")
	agentCount  = flag.Int("agentCount", 1, "number of agent nodes")
	ci          = flag.Bool("ci", false, "running on CI")
	tc          *docker.TestConfig
	pods        = make(map[string]pod)
)

func Test_DockerFlannel(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "Flannel Docker Test Suite")
}

var _ = Describe("Flannel Tests", Ordered, func() {
	Context("Setup Cluster", func() {
		It("should provision server and agent", func() {
			var err error
			tc, err = docker.NewTestConfig()
			Expect(err).NotTo(HaveOccurred())
			tc.ServerYaml = `
cni: flannel
node-label:
  "node-role=server"
`
			tc.AgentYaml = `
cni: flannel
node-label:
  "node-role=agent"
`
			Expect(tc.ProvisionServers(*serverCount)).To(Succeed())
			Expect(tc.ProvisionAgents(*agentCount)).To(Succeed())
			Expect(docker.RestartCluster(append(tc.Servers, tc.Agents...))).To(Succeed())
			Expect(tc.CopyAndModifyKubeconfig()).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(tests.CheckDefaultDeployments(tc.KubeconfigFile)).To(Succeed())
				g.Expect(tests.CheckDaemonSets([]string{"kube-flannel-ds", "rke2-ingress-nginx-controller"}, tc.KubeconfigFile)).To(Succeed())
			}, "240s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})

		It("should deploy test pods", func() {
			_, err := tc.DeployWorkload("flannel.yaml")
			Expect(err).NotTo(HaveOccurred(), "Workloads not deployed")
			Eventually(func(g Gomega) {
				getServerPodsCmd := "kubectl get pods -o custom-columns=NAME:.metadata.name -l node-role=server,k8s-app=nginx-app-flannel --field-selector=status.phase=Running --no-headers --kubeconfig=" + tc.KubeconfigFile
				getAgentPodsCmd := "kubectl get pods -o custom-columns=NAME:.metadata.name -l node-role=agent,k8s-app=nginx-app-flannel --field-selector=status.phase=Running --no-headers --kubeconfig=" + tc.KubeconfigFile

				serverCmdResponse, err := docker.RunCommand(getServerPodsCmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(serverCmdResponse).Should(ContainSubstring("nginx-app-flannel-server"), "flannel-test pod was not created")
				err = mapPods(serverCmdResponse, "server")
				g.Expect(err).NotTo(HaveOccurred())

				agentCmdResponse, err := docker.RunCommand(getAgentPodsCmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(agentCmdResponse).Should(ContainSubstring("nginx-app-flannel-agent"), "flannel-test pod was not created")
				err = mapPods(agentCmdResponse, "agent")
				g.Expect(err).NotTo(HaveOccurred())
			}, "60s", "5s").Should(Succeed())
		})
	})

	Context("Connectivity tests", func() {
		It("should allow inter pod communication on the same node", func() {
			cmd := "kubectl exec -it " + pods["server-1"].name + " --kubeconfig=" + tc.KubeconfigFile + " -- wget -O - http://" + pods["server-2"].ip + ":80"
			res, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("Welcome to nginx"))

			cmd = "kubectl exec -it " + pods["server-2"].name + " --kubeconfig=" + tc.KubeconfigFile + " -- wget -O - http://" + pods["server-1"].ip + ":80"
			res, err = docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("Welcome to nginx"))
		})

		It("should allow inter node pod communication", func() {
			cmd := "kubectl exec -it " + pods["server-1"].name + " --kubeconfig=" + tc.KubeconfigFile + " -- wget -O - http://" + pods["agent-1"].ip + ":80"
			res, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("Welcome to nginx"))

			cmd = "kubectl exec -it " + pods["agent-2"].name + " --kubeconfig=" + tc.KubeconfigFile + " -- wget -O - http://" + pods["server-2"].ip + ":80"
			res, err = docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("Welcome to nginx"))
		})

		It("should grant pods external access", func() {
			Eventually(func(g Gomega) {
				cmd := "wget -O - http://" + tc.Servers[0].IP + ":30080"
				res, err := docker.RunCommand(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).Should(ContainSubstring("Welcome to nginx"))
			}, "10s", "5s").Should(Succeed())
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
		AddReportEntry("journald-logs", tc.DumpServiceLogs(100))
		AddReportEntry("component-logs", tc.DumpComponentLogs(100))
	}
	if *ci || (tc != nil && !failed) {
		tc.Cleanup()
	}
})

func mapPods(response string, role string) error {
	podNames := strings.Split(strings.TrimSpace(response), "\n")

	podNumber := 1
	for _, podName := range podNames {
		key := role + "-" + strconv.Itoa(podNumber)
		getIpCmd := "kubectl get pod " + podName + " -o custom-columns=IP:.status.podIP --no-headers --kubeconfig=" + tc.KubeconfigFile
		res, err := docker.RunCommand(getIpCmd)
		if err != nil {
			return err
		}

		pods[key] = pod{
			ip:   strings.TrimSpace(res),
			name: podName,
		}

		podNumber = podNumber + 1
	}

	return nil
}
