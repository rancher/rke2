package validatecluster

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/e2e"
)

// Valid nodeOS: bento/ubuntu-24.04, opensuse/Leap-15.6.x86_64
var nodeOS = flag.String("nodeOS", "bento/ubuntu-24.04", "VM operating system")
var serverCount = flag.Int("serverCount", 3, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")
var ci = flag.Bool("ci", false, "running on CI")

// Environment Variables Info:
// E2E_CNI=(canal|cilium|calico)
// E2E_RELEASE_VERSION=v1.23.1+rke2r1 or nil for latest commit from master

func Test_E2EClusterValidation(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	suiteConfig, reporterConfig := GinkgoConfiguration()
	RunSpecs(t, "Validate Cluster Test Suite", suiteConfig, reporterConfig)
}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)
var _ = ReportAfterEach(e2e.GenReport)

var _ = Describe("Verify Basic Cluster Creation", Ordered, func() {

	It("Starts up with no issues", func() {
		var err error
		serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount)
		Expect(err).NotTo(HaveOccurred(), e2e.GetVagrantLog(err))
		fmt.Println("CLUSTER CONFIG")
		fmt.Println("OS:", *nodeOS)
		fmt.Println("Server Nodes:", serverNodeNames)
		fmt.Println("Agent Nodes:", agentNodeNames)
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

	It("Verifies ClusterIP Service", func() {
		_, err := e2e.DeployWorkload("clusterip.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() (string, error) {
			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
			return e2e.RunCommand(cmd)
		}, "240s", "5s").Should(ContainSubstring("test-clusterip"))

		clusterip, _ := e2e.FetchClusterIP(kubeConfigFile, "nginx-clusterip-svc", false)
		cmd := "curl -L --insecure http://" + clusterip + "/name.html"
		for _, nodeName := range serverNodeNames {
			Expect(e2e.RunCmdOnNode(cmd, nodeName)).Should(ContainSubstring("test-clusterip"), "failed cmd: "+cmd)
		}
	})

	It("Verifies NodePort Service", func() {
		_, err := e2e.DeployWorkload("nodeport.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, nodeName := range serverNodeNames {
			nodeExternalIP, err := e2e.FetchNodeExternalIP(nodeName)
			Expect(err).NotTo(HaveOccurred())
			cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport, err := e2e.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred(), "failed cmd: "+cmd)
			cmd = "curl -L --insecure http://" + nodeExternalIP + ":" + nodeport + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "5s", "1s").Should(ContainSubstring("test-nodeport"), "failed cmd: "+cmd)
			cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "120s", "5s").Should(ContainSubstring("test-nodeport"), "failed cmd: "+cmd)
		}
	})

	It("Verifies LoadBalancer Service", func() {
		_, err := e2e.DeployWorkload("loadbalancer.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		ip, err := e2e.FetchNodeExternalIP(serverNodeNames[0])
		Expect(err).NotTo(HaveOccurred(), "Loadbalancer manifest not deployed")
		cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
		port, err := e2e.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred())

		cmd = "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
		Eventually(func() (string, error) {
			return e2e.RunCommand(cmd)
		}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"))

		cmd = "curl -L --insecure http://" + ip + ":" + port + "/name.html"
		Eventually(func() (string, error) {
			return e2e.RunCommand(cmd)
		}, "240s", "5s").Should(ContainSubstring("test-loadbalancer"), "failed cmd: "+cmd)
	})

	It("Verifies Ingress", func() {
		_, err := e2e.DeployWorkload("ingress.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		for _, nodeName := range serverNodeNames {
			ip, _ := e2e.FetchNodeExternalIP(nodeName)
			cmd := "curl  --header host:foo1.bar.com" + " http://" + ip + "/name.html"
			Eventually(func() (string, error) {
				return e2e.RunCommand(cmd)
			}, "240s", "5s").Should(ContainSubstring("test-ingress"))
		}
	})

	It("Verifies Daemonset", func() {
		_, err := e2e.DeployWorkload("daemonset.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		nodes, err := e2e.ParseNodes(kubeConfigFile, false)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			pods, err := e2e.ParsePods(kubeConfigFile, false)
			g.Expect(err).NotTo(HaveOccurred())
			count := e2e.CountOfStringInSlice("test-daemonset", pods)
			g.Expect(len(nodes)).Should((Equal(count)), "Daemonset pod count does not match node count")
		}, "240s", "10s").Should(Succeed())
	})

	It("Verifies dns access", func() {
		_, err := e2e.DeployWorkload("dnsutils.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())
		cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
		Eventually(func() (string, error) {
			return e2e.RunCommand(cmd)
		}, "120s", "2s").Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
	})

	It("Verify Local Path Provisioner storage ", func() {
		_, err := e2e.DeployWorkload("local-path-provisioner.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() (string, error) {
			cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + kubeConfigFile
			return e2e.RunCommand(cmd)
		}, "120s", "2s").Should(MatchRegexp(`local-path-pvc.+Bound`))

		Eventually(func() (string, error) {
			cmd := "kubectl get pod volume-test --kubeconfig=" + kubeConfigFile
			return e2e.RunCommand(cmd)
		}, "420s", "2s").Should(MatchRegexp(`volume-test.+Running`))

		cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
		_, err = e2e.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred())

		cmd = "kubectl delete pod volume-test --kubeconfig=" + kubeConfigFile
		_, err = e2e.RunCommand(cmd)
		Expect(err).NotTo(HaveOccurred())

		_, err = e2e.DeployWorkload("local-path-provisioner.yaml", kubeConfigFile)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() (string, error) {
			cmd = "kubectl exec volume-test cat /data/test --kubeconfig=" + kubeConfigFile
			return e2e.RunCommand(cmd)
		}, "180s", "2s").Should(ContainSubstring("local-path-test"))
	})

	Context("Validate restart", func() {
		It("Restarts normally", func() {
			errRestart := e2e.RestartCluster(append(serverNodeNames, agentNodeNames...))
			Expect(errRestart).NotTo(HaveOccurred(), "Restart Nodes not happened correctly")

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
				pods, _ := e2e.ParsePods(kubeConfigFile, false)
				count := e2e.CountOfStringInSlice("test-daemonset", pods)
				g.Expect(len(nodes)).Should((Equal(count)), "Daemonset pod count does not match node count")
				podsRunningAr := 0
				for _, pod := range pods {
					if strings.Contains(pod.Name, "test-daemonset") && pod.Status == "Running" && pod.Ready == "1/1" {
						podsRunningAr++
					}
				}
				g.Expect(len(nodes)).Should((Equal(podsRunningAr)), "Daemonset pods are not running after the restart")
			}, "1120s", "5s").Should(Succeed())
		})
	})

	Context("Validate Certificate Rotation", func() {
		It("Stops rke2 and rotates certificates", func() {
			errStop := e2e.StopCluster(serverNodeNames)
			Expect(errStop).NotTo(HaveOccurred(), "Cluster could not be stopped successfully")

			for _, nodeName := range serverNodeNames {
				cmd := "sudo rke2 certificate rotate"
				if _, err := e2e.RunCmdOnNode(cmd, nodeName); err != nil {
					Expect(err).NotTo(HaveOccurred(), "Certificate could not be rotated successfully")
				}
			}
		})

		It("Start normally", func() {
			// Since we stopped all the server, we have to start 2 at once to get it back up
			// If we only start one at a time, the first will hang waiting for the second to be up
			_, err := e2e.RunCmdOnNode("sudo systemctl --no-block start rke2-server", serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
			err = e2e.StartCluster(serverNodeNames[1:])
			Expect(err).NotTo(HaveOccurred(), "Cluster could not be started successfully")

			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
				fmt.Println("help")
			}, "1120s", "5s").Should(Succeed())

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
			}, "1120s", "5s").Should(Succeed())
		})

		It("Validates certificates", func() {
			const grepCert = "sudo ls -lt /var/lib/rancher/rke2/server/ | grep tls"
			var expectResult = []string{"client-ca.crt",
				"client-ca.key",
				"client-ca.nochain.crt",
				"peer-ca.crt",
				"peer-ca.key", "server-ca.crt",
				"server-ca.key", "request-header-ca.crt",
				"request-header-ca.key", "server-ca.crt",
				"server-ca.key", "server-ca.nochain.crt",
				"service.current.key", "service.key",
				"",
			}

			var finalResult string
			var finalErr error
			for _, nodeName := range serverNodeNames {
				grCert, errGrep := e2e.RunCmdOnNode(grepCert, nodeName)
				Expect(errGrep).NotTo(HaveOccurred(), "Certificate could not be created successfully")
				re := regexp.MustCompile("tls-[0-9]+")
				tls := re.FindAllString(grCert, -1)[0]
				final := fmt.Sprintf("sudo diff -sr /var/lib/rancher/rke2/server/tls/ /var/lib/rancher/rke2/server/%s/"+
					"| grep -i identical | cut -f4 -d ' ' | xargs basename -a \n", tls)
				finalResult, finalErr = e2e.RunCmdOnNode(final, nodeName)
				Expect(finalErr).NotTo(HaveOccurred(), "Final Certification does not created successfully")
			}
			errRestartAgent := e2e.RestartCluster(agentNodeNames)
			Expect(errRestartAgent).NotTo(HaveOccurred(), "Agent could not be restart successfully")

			finalCert := strings.Replace(finalResult, "\n", ",", -1)
			finalCertArray := strings.Split(finalCert, ",")
			Expect((finalCertArray)).Should((Equal(expectResult)), "Final certification does not match the expected results")

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
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
