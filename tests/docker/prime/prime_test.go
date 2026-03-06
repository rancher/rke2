package main

import (
	"flag"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests"
	"github.com/rancher/rke2/tests/docker"
)

var (
	serverCount = flag.Int("serverCount", 1, "number of server nodes")
	agentCount  = flag.Int("agentCount", 1, "number of agent nodes")
	ci          = flag.Bool("ci", false, "running on CI, force cleanup")
	registry    = flag.String("registry", "", "prime registry to use for images")
	tc          *docker.TestConfig
)

func Test_DockerPrime(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "Prime Docker Test Suite")
}

var _ = Describe("Prime Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		It("should provision servers and agents", func() {
			var err error
			tc, err = docker.NewTestConfig()
			Expect(err).NotTo(HaveOccurred())
			tc.ServerYaml = "prime: true\n"
			Expect(tc.ProvisionServers(*serverCount)).To(Succeed())
			Expect(tc.ProvisionAgents(*agentCount)).To(Succeed())
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

	Context("Validate image registry values", func() {
		It("should have the default components come from the PRIME registry", func() {
			// Get all the containers image names in kube-system namespace
			cmd := "kubectl get pods -n kube-system -o jsonpath='{.items[*].spec.containers[*].image}' --kubeconfig=" + tc.KubeconfigFile
			res, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to get images from kube-system pods")
			images := strings.Split(res, " ")
			for _, image := range images {
				// The test binary we use still embeds the DockerHub klipper-lb image
				if strings.Contains(image, "klipper-helm") || strings.Contains(image, "rancher/hardened-kubernetes") {
					continue
				}
				Expect(image).To(ContainSubstring(*registry))
			}
		})
		It("should have a different rke2-ingress-nginx version", func() {
			cmd := "kubectl get daemonset -n kube-system rke2-ingress-nginx-controller -o jsonpath='{.spec.template.spec.containers[*].image}' --kubeconfig=" + tc.KubeconfigFile
			res, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to get ingress-nginx image")
			Expect(res).Should(MatchRegexp("nginx-ingress-controller.*?-prime[0-9]+"))
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
