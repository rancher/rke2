package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests"
	"github.com/rancher/rke2/tests/docker"
)

var serial = flag.Bool("serial", false, "Run the Serial Conformance Tests")
var ci = flag.Bool("ci", false, "running on CI, forced cleanup")
var tc *docker.TestConfig

func Test_DockerConformance(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conformance Docker Test Suite")
}

var _ = Describe("Conformance Tests", Ordered, func() {

	Context("Setup Cluster", func() {
		It("should provision servers and agents", func() {
			var err error
			tc, err = docker.NewTestConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(tc.ProvisionServers(1)).To(Succeed())
			Expect(tc.ProvisionAgents(1)).To(Succeed())
			Expect(docker.RestartCluster(append(tc.Servers, tc.Agents...))).To(Succeed())
			Expect(tc.CopyAndModifyKubeconfig()).To(Succeed())
			Eventually(func() error {
				return tests.CheckDefaultDeployments(tc.KubeconfigFile)
			}, "240s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
	})
	Context("Run Hydrophone Conformance tests", func() {
		It("should download hydrophone", func() {
			hydrophoneVersion := "v0.7.0"
			hydrophoneArch := runtime.GOARCH
			if hydrophoneArch == "amd64" {
				hydrophoneArch = "x86_64"
			}
			hydrophoneURL := fmt.Sprintf("https://github.com/kubernetes-sigs/hydrophone/releases/download/%s/hydrophone_Linux_%s.tar.gz",
				hydrophoneVersion, hydrophoneArch)
			cmd := fmt.Sprintf("curl -L %s | tar -xzf - -C %s", hydrophoneURL, tc.TestDir)
			_, err := docker.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Chmod(filepath.Join(tc.TestDir, "hydrophone"), 0755)).To(Succeed())
		})
		// Takes about 15min to run, so expect nothing to happen for a while
		It("should run parallel conformance tests", func() {
			if *serial {
				Skip("Skipping parallel conformance tests")
			}
			cmd := fmt.Sprintf("%s --focus=\"Conformance\" --skip=\"Serial|Flaky\" -v 2 -p %d --kubeconfig %s",
				filepath.Join(tc.TestDir, "hydrophone"),
				runtime.NumCPU()/2,
				tc.KubeconfigFile)
			By("Hydrophone: " + cmd)
			hc, err := StartCmd(cmd)
			Expect(err).NotTo(HaveOccurred())
			// Periodically check the number of tests that have run, since the hydrophone output does not support a progress status
			// Taken from https://github.com/kubernetes-sigs/hydrophone/issues/223#issuecomment-2547174722
			go func() {
				cmd := fmt.Sprintf("kubectl exec -n=conformance e2e-conformance-test -c output-container --kubeconfig=%s -- cat /tmp/results/e2e.log | grep -o \"•\" | wc -l",
					tc.KubeconfigFile)
				for i := 1; ; i++ {
					time.Sleep(120 * time.Second)
					if hc.ProcessState != nil {
						break
					}
					res, _ := docker.RunCommand(cmd)
					res = strings.TrimSpace(res)
					fmt.Printf("Status Report %d: %s tests complete\n", i, res)
				}
			}()
			Expect(hc.Wait()).To(Succeed())
		})
		It("should run serial conformance tests", func() {
			if !*serial {
				Skip("Skipping serial conformance tests")
			}
			cmd := fmt.Sprintf("%s --focus=\"\\[Serial\\].*\\[Conformance\\]\" --skip=\"Flaky\" -v 2 --kubeconfig %s",
				filepath.Join(tc.TestDir, "hydrophone"),
				tc.KubeconfigFile)
			By("Hydrophone: " + cmd)
			hc, err := StartCmd(cmd)
			Expect(err).NotTo(HaveOccurred())
			go func() {
				cmd := fmt.Sprintf("kubectl exec -n=conformance e2e-conformance-test -c output-container --kubeconfig=%s -- cat /tmp/results/e2e.log | grep -o \"•\" | wc -l",
					tc.KubeconfigFile)
				for i := 1; ; i++ {
					if hc.ProcessState != nil {
						break
					}
					time.Sleep(120 * time.Second)
					res, _ := docker.RunCommand(cmd)
					res = strings.TrimSpace(res)
					fmt.Printf("Status Report %d: %s tests complete\n", i, res)
				}
			}()
			Expect(hc.Wait()).To(Succeed())
		})
	})
})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		AddReportEntry("cluster-resources", tc.DumpResources())
	}
	if tc != nil && (*ci || !failed) {
		Expect(tc.Cleanup()).To(Succeed())
	}
})

// StartCmd starts a command and pipes its output to
// the ginkgo Writr, with the expectation to poll the progress of the command
func StartCmd(cmd string) (*exec.Cmd, error) {
	c := exec.Command("sh", "-c", cmd)
	c.Stdout = GinkgoWriter
	c.Stderr = GinkgoWriter
	if err := c.Start(); err != nil {
		return c, err
	}
	return c, nil
}
