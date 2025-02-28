package dnscache

import (
	"flag"
	"fmt"

	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/e2e"
)

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 1, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")
var local = flag.Bool("local", false, "deploy a locally built RKE2")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+rke2r1 or nil for latest commit from master

func Test_E2Ednscache(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Validate dnscache Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig
var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify dnscache Configuration", Ordered, func() {

	It("Starts up with no issues", func() {
		var err error
		if *local {
			tc, err = e2e.CreateLocalCluster(*nodeOS, *serverCount, *agentCount)
		} else {
			tc, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
		}
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
		By("CLUSTER CONFIG")
		By("OS: " + *nodeOS)
		By(tc.Status())
		tc.KubeconfigFile, err = e2e.GenKubeConfigFile(tc.Servers[0])
		Expect(err).NotTo(HaveOccurred())
	})

	It("Checks Node Status", func() {
		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"))
			}
		}, "620s", "5s").Should(Succeed())
		_, err := e2e.ParseNodes(tc.KubeconfigFile, true)
		Expect(err).NotTo(HaveOccurred())
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
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParsePods(tc.KubeconfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Verifies that each node has IPv4 and IPv6", func() {
		for _, node := range tc.Servers {
			cmd := fmt.Sprintf("kubectl get node %s -o jsonpath='{.status.addresses}' --kubeconfig=%s | jq '.[] | select(.type == \"ExternalIP\") | .address'",
				node.Name, tc.KubeconfigFile)
			res, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
			Expect(res).Should(ContainSubstring("10.10.10"))
			Expect(res).Should(ContainSubstring("fd11:decf:c0ff"))
		}
	})

	It("Verifies nodecache daemonset comes up", func() {
		_, err := tc.DeployWorkload("nodecache.yaml")
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() (string, error) {
			cmd := "kubectl get ds node-local-dns -n kube-system -o jsonpath='{.status.numberReady}' --kubeconfig=" + tc.KubeconfigFile
			return e2e.RunCommand(cmd)
		}, "120s", "5s").Should(ContainSubstring("2"))
	})

	It("Verifies nodecache is working", func() {
		cmd := "dig +retries=0 @169.254.20.10 www.kubernetes.io"
		for _, server := range tc.Servers {
			Expect(server.RunCmdOnNode(cmd)).Should(ContainSubstring("status: NOERROR"), "failed cmd: "+cmd)
		}
	})
})

var failed bool
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
