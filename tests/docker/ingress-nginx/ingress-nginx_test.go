package main

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/docker"
)

var (
	serverCount = flag.Int("serverCount", 1, "number of server nodes")
	agentCount  = flag.Int("agentCount", 1, "number of agent nodes")
	ci          = flag.Bool("ci", false, "running on CI, force cleanup")

	tc *docker.TestConfig
)

func Test_DockerIngressNginx(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "Ingress-NGINX Docker Test Suite")
}

var _ = Describe("Ingress-NGINX Tests", Ordered, func() {

	Context("Make sure cluster fails", func() {
		It("should provision servers and agents", func() {
			var err error
			tc, err = docker.NewTestConfig(GinkgoTB())
			Expect(err).NotTo(HaveOccurred())
			tc.ServerYaml = "ingress-controller: ingress-nginx"
			Expect(tc.ProvisionServers(*serverCount)).To(Succeed())
			Expect(tc.ProvisionAgents(*agentCount)).To(Succeed())
			Expect(docker.RestartCluster(append(tc.Servers, tc.Agents...))).To(Not(Succeed()))
			Expect(tc.DumpServiceLogs(50)).To(ContainSubstring("ingress-nginx is no longer supported as a standalone ingress controller"))
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
