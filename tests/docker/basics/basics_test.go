package main

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
	serverCount = flag.Int("serverCount", 1, "number of server nodes")
	agentCount  = flag.Int("agentCount", 1, "number of agent nodes")
	ci          = flag.Bool("ci", false, "running on CI, force cleanup")

	tc *docker.TestConfig
)

func Test_DockerBasic(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "Basic Docker Test Suite")
}

var _ = Describe("Basic Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		It("should provision servers and agents", func() {
			var err error
			tc, err = docker.NewTestConfig()
			Expect(err).NotTo(HaveOccurred())
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

	Context("Validate various components", func() {
		It("should deploy dns node cache", func() {
			_, err := tc.DeployWorkload("dns-node-cache.yaml")
			Expect(err).NotTo(HaveOccurred(), "failed to apply dns-node-cache manifest")
			Eventually(func() error {
				return tests.CheckDaemonSets([]string{"node-local-dns"}, tc.KubeconfigFile)
			}, "40s", "5s").Should(Succeed())
		})
		It("should deploy loadbalancer service", func() {
			_, err := tc.DeployWorkload("loadbalancer.yaml")
			Expect(err).NotTo(HaveOccurred(), "failed to apply loadbalancer manifest")
			Eventually(func(g Gomega) {
				sers, err := tests.ParseServices(tc.KubeconfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				foundLB := false
				for _, ser := range sers {
					if ser.Name == "lb-test" && ser.Namespace == "kube-system" {
						foundLB = true
						g.Expect(string(ser.Spec.Type)).To(Equal("LoadBalancer"))
						g.Expect(ser.Spec.Ports).To(HaveLen(2))
						if ser.Spec.Ports[0].Name == "http" {
							g.Expect(ser.Spec.Ports[0].Port).To(Equal(int32(8080)))
						} else {
							g.Expect(ser.Spec.Ports[1].Name).To(Equal("http"))
							g.Expect(ser.Spec.Ports[1].Port).To(Equal(int32(8080)))
						}
						if ser.Spec.Ports[0].Name == "https" {
							g.Expect(ser.Spec.Ports[0].Port).To(Equal(int32(8443)))
						} else {
							g.Expect(ser.Spec.Ports[1].Name).To(Equal("https"))
							g.Expect(ser.Spec.Ports[1].Port).To(Equal(int32(8443)))
						}
					}
				}
				g.Expect(foundLB).To(BeTrue())
			}, "30s", "5s").Should(Succeed())
		})
		It("should apply local storage volume", func() {
			localPathProvisionerURL := "https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml"
			cmd := fmt.Sprintf("kubectl apply -f %s --kubeconfig=%s", localPathProvisionerURL, tc.KubeconfigFile)
			_, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to apply local-path-storage manifest")
			_, err = tc.DeployWorkload("volume-test.yaml")
			Expect(err).NotTo(HaveOccurred(), "failed to apply volume test manifest")
		})
		It("should validate local storage volume", func() {
			Eventually(func() (bool, error) {
				return tests.PodReady("volume-test", "kube-system", tc.KubeconfigFile)
			}, "20s", "5s").Should(BeTrue())
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
