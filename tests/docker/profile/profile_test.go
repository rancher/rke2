package main

import (
	"flag"
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

func Test_DockerProfile(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "Profile Docker Test Suite")
}

var _ = Describe("Profile Tests", Ordered, func() {

	Context("Setup Cluster with profile: etcd", func() {
		It("should provision servers and agents", func() {
			var err error
			tc, err = docker.NewTestConfig()
			Expect(err).NotTo(HaveOccurred())
			tc.ServerYaml = "profile: etcd"
			Expect(tc.ProvisionServers(*serverCount)).To(Succeed())

			// Setup etcd user and group
			cmd := "useradd -r -c 'etcd user' -s /sbin/nologin -M etcd -U"
			_, err = tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed to create etcd user/group")

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

	Context("Validate etcd directories", func() {
		It("should have correct etcd directory permissions", func() {
			cmd := "stat -c permissions=%a /var/lib/rancher/rke2/server/db/etcd"
			for _, server := range tc.Servers {
				out, err := server.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(Equal("permissions=700\n"))
			}
		})
		It("should have correct etcd manifest permissions", func() {
			cmd := "stat -c permissions=%a /var/lib/rancher/rke2/agent/pod-manifests/etcd.yaml"
			for _, server := range tc.Servers {
				out, err := server.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(Equal("permissions=600\n"))
			}
		})
		It("should have correct etcd ownership", func() {
			cmd := "stat -c %U:%G /var/lib/rancher/rke2/server/db/etcd"
			for _, server := range tc.Servers {
				out, err := server.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(Equal("etcd:etcd\n"))
			}
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
