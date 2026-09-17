package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests"
	"github.com/rancher/rke2/tests/docker"
)

var (
	channel = flag.String("channel", "latest", "RKE2 release channel to upgrade from")
	ci      = flag.Bool("ci", false, "running on CI, force cleanup")
	tc      *docker.TestConfig
)

const localImageFile = "../../../build/images.txt"

func Test_DockerUpgrade(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()
	RunSpecs(t, "Upgrade Docker Test Suite")
}

var _ = Describe("Upgrade Tests", Ordered, func() {
	Context("Setup cluster with channel release", func() {
		It("should provision servers and agents", func() {
			var err error
			tc, err = docker.NewTestConfig(GinkgoTB())
			Expect(err).NotTo(HaveOccurred())
			tc.SkipInstall = true
			tc.ServerYaml = "enable-servicelb: true"
			Expect(tc.ProvisionServers(1)).To(Succeed())
			Expect(tc.ProvisionAgents(1)).To(Succeed())
			Expect(tc.Install(docker.InstallOptions{Channel: *channel})).To(Succeed())
			// Should use the rancher/hardened-addon-resizer image
			metricAddonManifest := `
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-metrics-server
  namespace: kube-system
spec:
  valuesContent: |
    addonResizer:
      enabled: true
`
			_, err = docker.StageManifest(metricAddonManifest, tc.Servers)
			Expect(err).To(Succeed())
			Expect(docker.RestartCluster(append(tc.Servers, tc.Agents...))).To(Succeed())
			Expect(tc.CopyAndModifyKubeconfig()).To(Succeed())
		})
		It("checks cluster is ready", func() {
			Eventually(func(g Gomega) {
				g.Expect(tests.CheckDefaultDeployments(tc.KubeconfigFile)).To(Succeed())
				g.Expect(tests.CheckDefaultDaemonSets(tc.KubeconfigFile)).To(Succeed())
			}, "240s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
		// Should use the rancher/hardned-dns-node-cache image
		It("should deploy dns node cache", func() {
			_, err := tc.DeployWorkload("dns-node-cache.yaml")
			Expect(err).NotTo(HaveOccurred(), "failed to apply dns-node-cache manifest")
			Eventually(func() error {
				return tests.CheckDaemonSets([]string{"node-local-dns"}, tc.KubeconfigFile)
			}, "40s", "5s").Should(Succeed())
		})
		// Should trigger use of the rancher/klipper-lb image
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
		It("validates the release cluster images", func() {
			version, err := getVersionFromChannel(*channel)
			Expect(err).NotTo(HaveOccurred())
			expectedImages := filepath.Join(tc.TestDir, "release-images.txt")
			releaseURL := fmt.Sprintf("https://github.com/rancher/rke2/releases/download/%s/rke2-images.linux-amd64.txt", version)
			_, err = docker.RunCommand(fmt.Sprintf("curl -fsSL -o %s %s", expectedImages, releaseURL))
			Expect(err).NotTo(HaveOccurred())
			contents, err := os.ReadFile(expectedImages)
			Expect(err).NotTo(HaveOccurred())
			expected := imageList(contents)
			imagePods, err := kubeSystemPodImages(tc.KubeconfigFile)
			Expect(err).NotTo(HaveOccurred())
			for _, image := range expected {
				if strings.Contains(image, "rancher/rke2-runtime") ||
					strings.Contains(image, "rancher/mirrored-pause") ||
					strings.Contains(image, "rancher/rke2-security-responder") {
					continue
				}
				Expect(imagePods).To(HaveKey(image), "no kube-system pod is using image %s", image)
			}
		})
	})

	Context("Upgrade cluster to commit build", func() {
		It("upgrades to local build version", func() {
			Expect(tc.Install(docker.InstallOptions{ArtifactPath: "/src/rke2-artifacts"})).To(Succeed())
			Expect(docker.RestartCluster(append(tc.Servers, tc.Agents...))).To(Succeed())
		})
		It("check upgraded cluster is ready", func() {
			Eventually(func(g Gomega) {
				g.Expect(tests.CheckDefaultDeployments(tc.KubeconfigFile)).To(Succeed())
				g.Expect(tests.CheckDefaultDaemonSets(tc.KubeconfigFile)).To(Succeed())
			}, "240s", "5s").Should(Succeed())
			Eventually(func() error {
				return tests.NodesReady(tc.KubeconfigFile, tc.GetNodeNames())
			}, "40s", "5s").Should(Succeed())
		})
		It("validates the upgraded cluster images", func() {
			contents, err := os.ReadFile(localImageFile)
			Expect(err).NotTo(HaveOccurred())
			expected := imageList(contents)
			imagePods, err := kubeSystemPodImages(tc.KubeconfigFile)
			Expect(err).NotTo(HaveOccurred())
			for _, image := range expected {
				if strings.Contains(image, "rancher/rke2-runtime") ||
					strings.Contains(image, "rancher/mirrored-pause") ||
					strings.Contains(image, "rancher/rke2-security-responder") {
					continue
				}
				Expect(imagePods).To(HaveKey(image), "no kube-system pod is using image %s", image)
			}
		})
	})
})

func getVersionFromChannel(channel string) (string, error) {
	client := &http.Client{}
	resp, err := client.Get("https://update.rke2.io/v1-release/channels/" + channel)
	if err != nil {
		return "", fmt.Errorf("failed to get release channel %q: %w", channel, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("unexpected status code from release channel %q: %d", channel, resp.StatusCode)
	}
	if resp.Request == nil || resp.Request.URL == nil {
		return "", fmt.Errorf("release channel %q did not provide a release location", channel)
	}
	version := path.Base(resp.Request.URL.Path)
	if !strings.HasPrefix(version, "v1.") {
		return "", fmt.Errorf("release channel %q returned invalid version %q", channel, version)
	}
	return version, nil
}

func imageList(contents []byte) []string {
	var expected []string
	for _, line := range strings.Split(string(contents), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "docker.io/")
		if line != "" {
			expected = append(expected, line)
		}
	}
	return expected
}

func kubeSystemPodImages(kubeconfig string) (map[string][]string, error) {
	pods, err := tests.ParsePods(kubeconfig)
	if err != nil {
		return nil, err
	}
	imagePod := make(map[string][]string)
	for _, pod := range pods {
		if pod.Namespace != "kube-system" {
			continue
		}
		for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
			// Etcd and CCM have this prefix, so we trim it to normalize the image names.
			image := strings.TrimPrefix(container.Image, "index.docker.io/")
			imagePod[image] = append(imagePod[image], pod.Name)
		}
	}
	return imagePod, nil
}

var failed bool

var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if tc != nil && tc.TestDir != "" {
		_ = os.Remove(filepath.Join(tc.TestDir, "release-images.txt"))
	}
	if tc != nil && failed {
		logLen := 10
		if *ci {
			logLen = 200
		}
		AddReportEntry("cluster-resources", tc.DumpResources())
		AddReportEntry("pod-logs", tc.DumpPodLogs(logLen))
		AddReportEntry("journald-logs", tc.DumpServiceLogs(logLen))
		AddReportEntry("component-logs", tc.DumpComponentLogs(logLen))
	}
	if tc != nil && (*ci || !failed) {
		Expect(tc.Cleanup()).To(Succeed())
	}
})
