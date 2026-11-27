package repo

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/docker"
)

const (
	installSh  = "../../../install.sh"
	installEnv = "INSTALL_RKE2_CHANNEL=testing " +
		"INSTALL_RKE2_TYPE=server"
)

var ci = flag.Bool("ci", false, "running on CI, forced cleanup")

type distro struct {
	image       string
	repoFile    string
	wantBaseURL string
}

func Test_DockerRepo(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Repository Docker Test Suite")
}

func installScript(d distro) string {
	return strings.Join([]string{
		"exec 2>&1",
		installEnv + " INSTALL_RKE2_METHOD=rpm sh /tmp/install.sh",
		fmt.Sprintf(`echo "REPO: $(test -f %[1]s && grep -h '^baseurl=' %[1]s | head -1 || echo missing)"`, d.repoFile),
	}, "\n")
}

func runScript(name, image, script string) (*docker.TestConfig, string, error) {
	tc, err := docker.NewTestConfig(GinkgoTB())
	if err != nil {
		return nil, "", err
	}
	name = fmt.Sprintf("repo-%s-%s", name, strings.ToLower(filepath.Base(tc.TestDir)))

	installShPath, err := filepath.Abs(installSh)
	if err != nil {
		return tc, "", err
	}
	scriptPath := filepath.Join(tc.TestDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return tc, "", err
	}

	tc.Servers = append(tc.Servers, docker.DockerNode{Name: name})
	dRun := strings.Join([]string{"timeout 900 docker run",
		"--name", name,
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/tmp/install.sh,ro", installShPath),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/tmp/test.sh,ro", scriptPath),
		image, "/bin/sh", "/tmp/test.sh"}, " ")
	out, err := docker.RunCommand(dRun)
	if err != nil {
		return tc, out, fmt.Errorf("failed to run %s: %v", name, err)
	}
	return tc, out, nil
}

var _ = DescribeTableSubtree("Repository Tests", Ordered, func(d distro) {
	var (
		tc     *docker.TestConfig
		out    string
		failed bool
	)

	BeforeAll(func() {
		var err error
		texts := CurrentSpecReport().ContainerHierarchyTexts
		name := texts[len(texts)-1]
		tc, out, err = runScript(name, d.image, installScript(d))
		Expect(err).NotTo(HaveOccurred(), out)
	})

	It("should configure the expected repository", func() {
		Expect(out).To(ContainSubstring("REPO: baseurl=" + d.wantBaseURL))
	})

	AfterEach(func() {
		failed = failed || CurrentSpecReport().Failed()
	})

	AfterAll(func() {
		cleanup(tc, failed)
	})
},
	Entry("sles16", distro{
		image:       "registry.suse.com/bci/bci-base:16.0",
		repoFile:    "/etc/zypp/repos.d/rancher-rke2.repo",
		wantBaseURL: "https://rpm-testing.rancher.io/rke2/testing/common/sle/noarch",
	}),
	Entry("opensuse-leap16", distro{
		image:       "opensuse/leap:16.0",
		repoFile:    "/etc/zypp/repos.d/rancher-rke2.repo",
		wantBaseURL: "https://rpm-testing.rancher.io/rke2/testing/common/sle/noarch",
	}),
	Entry("sl-micro-6.2", distro{
		image:       "registry.suse.com/suse/sl-micro/6.2/base-os-container:latest",
		repoFile:    "/etc/zypp/repos.d/rancher-rke2.repo",
		wantBaseURL: "https://rpm-testing.rancher.io/rke2/testing/common/sle/noarch",
	}),
	Entry("sl-micro-6.1", distro{
		image:       "registry.suse.com/suse/sl-micro/6.1/base-os-container:latest",
		repoFile:    "/etc/zypp/repos.d/rancher-rke2.repo",
		wantBaseURL: "https://rpm-testing.rancher.io/rke2/testing/common/sle/noarch",
	}),
	Entry("sl-micro-6.0", distro{
		image:       "registry.suse.com/suse/sl-micro/6.0/base-os-container:latest",
		repoFile:    "/etc/zypp/repos.d/rancher-rke2.repo",
		wantBaseURL: "https://rpm-testing.rancher.io/rke2/testing/common/sle/noarch",
	}),
	Entry("sle-micro", distro{
		image:       "registry.suse.com/suse/sle-micro/5.5:latest",
		repoFile:    "/etc/zypp/repos.d/rancher-rke2.repo",
		wantBaseURL: "https://rpm-testing.rancher.io/rke2/testing/common/slemicro/noarch",
	}),
	Entry("almalinux", distro{
		image:       "almalinux:10",
		repoFile:    "/etc/yum.repos.d/rancher-rke2.repo",
		wantBaseURL: "https://rpm-testing.rancher.io/rke2/testing/common/centos/10/noarch",
	}),
	Entry("centos", distro{
		image:       "quay.io/centos/centos:stream9",
		repoFile:    "/etc/yum.repos.d/rancher-rke2.repo",
		wantBaseURL: "https://rpm-testing.rancher.io/rke2/testing/common/centos/9/noarch",
	}),
	Entry("rockylinux", distro{
		image:       "rockylinux/rockylinux:8",
		repoFile:    "/etc/yum.repos.d/rancher-rke2.repo",
		wantBaseURL: "https://rpm-testing.rancher.io/rke2/testing/common/centos/8/noarch",
	}),
)

func cleanup(tc *docker.TestConfig, failed bool) {
	if failed {
		AddReportEntry("docker-logs", docker.TailDockerLogs(1000, tc.Servers))
	}
	if tc != nil && (*ci || !failed) {
		Expect(tc.Cleanup()).To(Succeed())
	}
}
