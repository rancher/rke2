package secretsencryption

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
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var ci = flag.Bool("ci", false, "running on CI")

// Environment Variables Info:
// E2E_RELEASE_VERSION=v1.23.1+rke2r1 or nil for latest commit from master

func Test_E2ESecretsEncryptionOld(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Secrets Encryption Test Suite", suiteConfig, reporterConfig)
}

var tc *e2e.TestConfig

var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Secrets Encryption Rotation", Ordered, func() {
	Context("Secrets Keys are rotated:", func() {
		It("Starts up with no issues", func() {
			var err error
			tc, err = e2e.CreateCluster(*nodeOS, *serverCount, 0)
			Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
			By("CLUSTER CONFIG")
			By("OS: " + *nodeOS)
			By(tc.Status())
			tc.KubeconfigFile, err = e2e.GenKubeConfigFile(tc.Servers[0])
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks node and pod status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParseNodes(tc.KubeconfigFile, true)

			fmt.Printf("\nFetching pods status\n")
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
			}, "620s", "5s").Should(Succeed())
			_, _ = e2e.ParsePods(tc.KubeconfigFile, true)
		})

		It("Deploys several secrets", func() {
			_, err := tc.DeployWorkload("secrets.yaml")
			Expect(err).NotTo(HaveOccurred(), "Secrets not deployed")
		})

		It("Verifies encryption start stage", func() {
			cmd := "sudo rke2 secrets-encrypt status"
			for _, server := range tc.Servers {
				res, err := server.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred())
				Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
				Expect(res).Should(ContainSubstring("Current Rotation Stage: start"))
				Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
			}
		})

		It("Prepares for Secrets-Encryption Rotation", func() {
			cmd := "sudo rke2 secrets-encrypt prepare"
			res, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
			for i, server := range tc.Servers {
				cmd := "sudo rke2 secrets-encrypt status"
				res, err := server.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred(), res)
				Expect(res).Should(ContainSubstring("Server Encryption Hashes: hash does not match"))
				if i == 0 {
					Expect(res).Should(ContainSubstring("Current Rotation Stage: prepare"))
				} else {
					Expect(res).Should(ContainSubstring("Current Rotation Stage: start"))
				}
			}
		})

		It("Restarts RKE2 servers", func() {
			Expect(e2e.RestartCluster(tc.Servers)).To(Succeed(), e2e.GetVagrantLog(nil))
		})

		It("Checks node and pod status", func() {
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(tc.KubeconfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())

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
			_, _ = e2e.ParseNodes(tc.KubeconfigFile, true)
		})

		It("Verifies encryption prepare stage", func() {
			cmd := "sudo rke2 secrets-encrypt status"
			for _, server := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := server.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					g.Expect(res).Should(ContainSubstring("Current Rotation Stage: prepare"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Rotates the Secrets-Encryption Keys", func() {
			cmd := "sudo rke2 secrets-encrypt rotate"
			res, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)
			for i, server := range tc.Servers {
				Eventually(func(g Gomega) {
					cmd := "sudo rke2 secrets-encrypt status"
					res, err := server.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred(), res)
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: hash does not match"))
					if i == 0 {
						g.Expect(res).Should(ContainSubstring("Current Rotation Stage: rotate"))
					} else {
						g.Expect(res).Should(ContainSubstring("Current Rotation Stage: prepare"))
					}
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Restarts RKE2 servers", func() {
			Expect(e2e.RestartCluster(tc.Servers)).To(Succeed(), e2e.GetVagrantLog(nil))
		})

		It("Verifies encryption rotate stage", func() {
			cmd := "sudo rke2 secrets-encrypt status"
			for _, server := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := server.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					g.Expect(res).Should(ContainSubstring("Current Rotation Stage: rotate"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "420s", "2s").Should(Succeed())
			}
		})

		It("Reencrypts the Secrets-Encryption Keys", func() {
			cmd := "sudo rke2 secrets-encrypt reencrypt"
			res, err := tc.Servers[0].RunCmdOnNode(cmd)
			Expect(err).NotTo(HaveOccurred(), res)

			cmd = "sudo rke2 secrets-encrypt status"
			Eventually(func() (string, error) {
				return tc.Servers[0].RunCmdOnNode(cmd)
			}, "240s", "10s").Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))

			for _, server := range tc.Servers[1:] {
				res, err := server.RunCmdOnNode(cmd)
				Expect(err).NotTo(HaveOccurred(), res)
				Expect(res).Should(ContainSubstring("Server Encryption Hashes: hash does not match"))
				Expect(res).Should(ContainSubstring("Current Rotation Stage: rotate"))
			}
		})

		It("Restarts RKE2 Servers", func() {
			Expect(e2e.RestartCluster(tc.Servers)).To(Succeed(), e2e.GetVagrantLog(nil))
		})

		It("Verifies Encryption Reencrypt Stage", func() {
			cmd := "sudo rke2 secrets-encrypt status"
			for _, server := range tc.Servers {
				Eventually(func(g Gomega) {
					res, err := server.RunCmdOnNode(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("Encryption Status: Enabled"))
					g.Expect(res).Should(ContainSubstring("Current Rotation Stage: reencrypt_finished"))
					g.Expect(res).Should(ContainSubstring("Server Encryption Hashes: All hashes match"))
				}, "420s", "5s").Should(Succeed())
			}
		})
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
