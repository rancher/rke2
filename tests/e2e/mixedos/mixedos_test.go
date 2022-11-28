package mixedos

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/e2e"
)

// Valid nodeOS: generic/ubuntu2004, opensuse/Leap-15.3.x86_64
var nodeOS = flag.String("nodeOS", "generic/ubuntu2004", "operating system for linux nodes")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var linuxAgentCount = flag.Int("linuxAgentCount", 0, "number of linux agent nodes")
var windowsAgentCount = flag.Int("windowsAgentCount", 1, "number of windows agent nodes")

const defaultWindowsOS = "jborean93/WindowsServer2022"

func Test_E2EMixedOSValidation(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Validate Cluster Suite")
}

func createMixedCluster(nodeOS string, serverCount, linuxAgentCount, windowsAgentCount int) ([]string, []string, []string, error) {
	serverNodeNames := []string{}
	for i := 0; i < serverCount; i++ {
		serverNodeNames = append(serverNodeNames, "server-"+strconv.Itoa(i))
	}
	linuxAgentNames := []string{}
	for i := 0; i < linuxAgentCount; i++ {
		linuxAgentNames = append(linuxAgentNames, "linux-agent-"+strconv.Itoa(i))
	}
	windowsAgentNames := []string{}
	for i := 0; i < linuxAgentCount; i++ {
		windowsAgentNames = append(windowsAgentNames, "linux-agent-"+strconv.Itoa(i))
	}
	nodeRoles := strings.Join(serverNodeNames, " ") + " " + strings.Join(linuxAgentNames, " ") + " " + strings.Join(windowsAgentNames, " ")
	nodeRoles = strings.TrimSpace(nodeRoles)
	nodeBoxes := strings.Repeat(nodeOS+" ", serverCount+linuxAgentCount)
	nodeBoxes += strings.Repeat(defaultWindowsOS+" ", windowsAgentCount)
	nodeBoxes = strings.TrimSpace(nodeBoxes)

	var testOptions string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}

	cmd := fmt.Sprintf("NODE_ROLES=\"%s\" NODE_BOXES=\"%s\" %s vagrant up &> vagrant.log", nodeRoles, nodeBoxes, testOptions)
	fmt.Println(cmd)
	if _, err := e2e.RunCommand(cmd); err != nil {
		fmt.Println("Error Creating Cluster", err)
		return nil, nil, nil, err
	}
	return serverNodeNames, linuxAgentNames, windowsAgentNames, nil
}

var (
	kubeConfigFile    string
	serverNodeNames   []string
	linuxAgentNames   []string
	windowsAgentNames []string
)

var _ = Describe("Verify Basic Cluster Creation", Ordered, func() {

	It("Starts up with no issues", func() {
		var err error
		serverNodeNames, linuxAgentNames, windowsAgentNames, err = createMixedCluster(*nodeOS, *serverCount, *linuxAgentCount, *windowsAgentCount)
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
		fmt.Println("CLUSTER CONFIG")
		fmt.Println("OS:", *nodeOS)
		fmt.Println("Server Nodes:", serverNodeNames)
		fmt.Println("Agent Nodes:", linuxAgentNames)
		kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred())
	})

	It("Checks Node Status", func() {
		Eventually(func(g Gomega) {
			nodes, err := e2e.ParseNodes(kubeConfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, node := range nodes {
				g.Expect(node.Status).Should(Equal("Ready"))
			}
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParseNodes(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Checks Pod Status", func() {
		Eventually(func(g Gomega) {
			pods, err := e2e.ParsePods(kubeConfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods {
				if strings.Contains(pod.Name, "helm-install") {
					g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
				} else {
					g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
				}
			}
		}, "420s", "5s").Should(Succeed())
		_, err := e2e.ParsePods(kubeConfigFile, true)
		Expect(err).NotTo(HaveOccurred())
	})
	It("Verifies internode connectivity over the vxlan tunnel", func() {
		_, err := e2e.DeployWorkload("pod_client.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())

		_, err = e2e.DeployWorkload("windows_app_deployment.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())

		// Wait for the pod_client pods to have an IP
		Eventually(func() string {
			ips, _ := e2e.PodIPsUsingLabel(kubeConfigFile, "app=client")
			return ips[0]
		}, "60s", "10s").Should(ContainSubstring("10.42"), "failed getClientIPs")

		// Wait for the windows_app_deployment pods to have an IP (We must wait 250s because it takes time)
		Eventually(func() string {
			ips, _ := e2e.PodIPsUsingLabel(kubeConfigFile, "app=windows-app")
			return ips[0]
		}, "250s", "10s").Should(ContainSubstring("10.42"), "failed getClientIPs")

		// Test Linux -> Windows communication
		cmd := "kubectl exec svc/client-curl --kubeconfig=" + kubeConfigFile + " -- curl -m7 windows-app-svc:3000"
		Eventually(func() (string, error) {
			return e2e.RunCommand(cmd)
		}, "20s", "3s").Should(ContainSubstring("Welcome to PSTools for K8s Debugging"), "failed cmd: "+cmd)

		// Test Windows -> Linux communication
		cmd = "kubectl exec svc/windows-app-svc --kubeconfig=" + kubeConfigFile + " -- curl -m7 client-curl:8080"
		Eventually(func() (string, error) {
			return e2e.RunCommand(cmd)
		}, "20s", "3s").Should(ContainSubstring("Welcome to nginx!"), "failed cmd: "+cmd)
	})
	It("Runs the mixed os sonobuoy plugin", func() {
		cmd := "sonobuoy run --kubeconfig=/etc/rancher/rke2/rke2.yaml --plugin my-sonobuoy-plugins/mixed-workload-e2e/mixed-workload-e2e.yaml --aggregator-node-selector kubernetes.io/os:linux --wait"
		res, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred(), "failed output:"+res)
		cmd = "sonobuoy retrieve --kubeconfig=/etc/rancher/rke2/rke2.yaml"
		testResultTar, err := e2e.RunCmdOnNode(cmd, serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		cmd = "sonobuoy results " + testResultTar
		res, err = e2e.RunCmdOnNode(cmd, serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
		Expect(res).Should(ContainSubstring("Plugin: mixed-workload-e2e\nStatus: passed\n"))
	})

})

var failed bool
var _ = AfterEach(func() {
	failed = failed || CurrentSpecReport().Failed()
})

var _ = AfterSuite(func() {
	if failed {
		fmt.Println("FAILED!")
	} else {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
