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
	ci          = flag.Bool("ci", false, "running on CI, force cleanup")

	tc *docker.TestConfig
)

func Test_DockerGatewayAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "Gateway API Docker Test Suite")
}

var _ = Describe("Gateway API Tests", Ordered, func() {
	Context("Setup Cluster", func() {
		It("should provision servers with Traefik Gateway API enabled", func() {
			var err error
			tc, err = docker.NewTestConfig(GinkgoTB())
			Expect(err).NotTo(HaveOccurred())

			Expect(tc.ProvisionServers(*serverCount)).To(Succeed())

			_, err = docker.EnableTraefikGatewayAPI(tc.Servers)
			Expect(err).NotTo(HaveOccurred())

			Expect(docker.RestartCluster(tc.Servers)).To(Succeed())
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

	Context("Validate Gateway API", func() {
		It("should install Gateway API CRDs from the standalone chart", func() {
			cmd := "kubectl get crd gateways.gateway.networking.k8s.io httproutes.gateway.networking.k8s.io gatewayclasses.gateway.networking.k8s.io --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "300s", "5s").Should(ContainSubstring("gatewayclasses.gateway.networking.k8s.io"), "failed cmd: "+cmd)

			cmd = "kubectl get crd gateways.gateway.networking.k8s.io httproutes.gateway.networking.k8s.io gatewayclasses.gateway.networking.k8s.io -o jsonpath='{range .items[*]}{.metadata.name}={.metadata.annotations.meta\\.helm\\.sh/release-name}{\"\\n\"}{end}' --kubeconfig=" + tc.KubeconfigFile
			Eventually(func(g Gomega) {
				out, err := docker.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
				crdReleases := strings.Fields(out)
				g.Expect(crdReleases).To(HaveLen(3))
				g.Expect(crdReleases).To(HaveEach(MatchRegexp(`^[^=]+=rke2-gateway-api-crd$`)))
			}, "120s", "5s").Should(Succeed())
		})

		It("should route HTTP traffic through Traefik Gateway API", func() {
			_, err := tc.DeployWorkload("gatewayapi.yaml")
			Expect(err).NotTo(HaveOccurred(), "Gateway API workload manifest not deployed")

			cmd := "kubectl get pods -o=name -l k8s-app=gatewayapi-echo --field-selector=status.phase=Running --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("gatewayapi-echo"), "gatewayapi-echo pod was not created")

			cmd = "kubectl get gatewayclass rke2-traefik -o jsonpath='{.status.conditions[?(@.type==\"Accepted\")].status}' --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "120s", "5s").Should(Equal("True"), "GatewayClass was not accepted")

			cmd = "kubectl get httproute gatewayapi-echo -o jsonpath='{.status.parents[0].conditions[?(@.type==\"Accepted\")].status}' --kubeconfig=" + tc.KubeconfigFile
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "120s", "5s").Should(Equal("True"), "HTTPRoute was not accepted")

			cmd = "curl -s -o /dev/null --max-time 10 -w '%{http_code}' -H 'Host: gatewayapi.example.com' http://" + tc.Servers[0].IP
			Eventually(func() (string, error) {
				return docker.RunCommand(cmd)
			}, "120s", "5s").Should(Equal("200"), "failed to curl gatewayapi.example.com")
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
