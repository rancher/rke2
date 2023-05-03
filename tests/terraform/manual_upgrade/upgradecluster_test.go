package manual_upgrade

import (
	"flag"
	"fmt"
	"regexp"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rancher/rke2/tests/terraform"
	"github.com/rancher/rke2/tests/terraform/createcluster"
)

var tfVars = flag.String("tfvars", "/tests/terraform/modules/config/local.tfvars", "custom .tfvars file from base project path")
var destroy = flag.Bool("destroy", false, "a bool")
var upgradeVersion = flag.String("upgradeVersion", "", "Version to upgrade the cluster to")
var channel = flag.String("channel", "", "Channel to use inside the installing command")
var failed bool

func Test_TFUpgradeClusterValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()

	RunSpecs(t, "Upgrade Cluster Test Suite")
}

var _ = Describe("Upgrade Tests:", func() {
	Context("Build Cluster:", func() {
		It("Starts up with no issues", func() {
			status, err := createcluster.BuildCluster(&testing.T{}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal("cluster created"))
			defer GinkgoRecover()
			fmt.Println("Server Node IPS:", createcluster.MasterIPs)
			fmt.Println("Agent Node IPS:", createcluster.WorkerIPs)
			terraform.PrintFileContents(createcluster.KubeConfigFile)
			Expect(createcluster.MasterIPs).ShouldNot(BeEmpty())
			if createcluster.NumWorkers > 0 {
				Expect(createcluster.WorkerIPs).ShouldNot(BeEmpty())
			} else {
				Expect(createcluster.WorkerIPs).Should(BeEmpty())
			}
			Expect(createcluster.KubeConfigFile).ShouldNot(BeEmpty())
		})

		It("Checks Node and Pod Status", func() {
			defer func() {
				_, err := terraform.Nodes(createcluster.KubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving nodes preupgrade: ", err)
				}
				_, err = terraform.Pods(createcluster.KubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving pods preupgrade: ", err)
				}
			}()

			fmt.Printf("\nFetching node status preupgrade\n")
			expectedNodeCount := createcluster.NumServers + createcluster.NumWorkers
			Eventually(func(g Gomega) {
				nodes, err := terraform.Nodes(createcluster.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).To(Equal(expectedNodeCount), "Number of nodes should match the spec")
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"), "Nodes should all be in Ready state")
				}
			}, "420s", "5s").Should(Succeed())

			re := regexp.MustCompile("[0-9]+")
			fmt.Printf("\nFetching pod status preupgrade\n")
			Eventually(func(g Gomega) {
				pods, err := terraform.Pods(createcluster.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
						g.Expect(pod.Restarts).Should(Equal("0"), pod.Name)
						numRunning := re.FindAllString(pod.Ready, 2)
						g.Expect(numRunning[0]).Should(Equal(numRunning[1]), pod.Name, "should have all containers running")
					}
				}
			}, "600s", "5s").Should(Succeed())
		})
	})

	Context("Preupgrade Validations:", func() {
		It("Verifies ClusterIP Service Preupgrade", func() {
			namespace := "auto-clusterip"
			_, err := terraform.DeployWorkload("clusterip.yaml", createcluster.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			Eventually(func(g Gomega) {
				res, err := terraform.IsAppRunning(namespace, "nginx-app-clusterip", createcluster.KubeConfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should((ContainSubstring("test-clusterip")))
			}, "420s", "5s").Should(Succeed())

			clusterip, port, _ := terraform.FetchClusterIP(createcluster.KubeConfigFile, namespace, "nginx-clusterip-svc")
			cmd := "curl -sL --insecure http://" + clusterip + ":" + port + "/name.html"
			nodeExternalIP := terraform.FetchNodeExternalIP(createcluster.KubeConfigFile)
			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					res, err := terraform.RunCommandOnNode(cmd, ip, createcluster.AwsUser, createcluster.AccessKey)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-clusterip"))
				}, "420s", "10s").Should(Succeed())
			}
		})

		It("Verifies NodePort Service Preupgrade", func() {
			namespace := "auto-nodeport"
			_, err := terraform.DeployWorkload("nodeport.yaml", createcluster.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")

			nodeExternalIP := terraform.FetchNodeExternalIP(createcluster.KubeConfigFile)
			cmd := "kubectl get service -n " + namespace + " nginx-nodeport-svc --kubeconfig=" + createcluster.KubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport, err := terraform.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					res, err := terraform.IsAppRunning(namespace, "nginx-app-nodeport", createcluster.KubeConfigFile)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "240s", "5s").Should(Succeed())

				cmd = "curl -sL --insecure http://" + ip + ":" + nodeport + "/name.html"
				Eventually(func(g Gomega) {
					res, err := terraform.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Ingress Preupgrade", func() {
			namespace := "auto-ingress"
			_, err := terraform.DeployWorkload("ingress.yaml", createcluster.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")

			Eventually(func(g Gomega) {
				res, err := terraform.IsAppRunning(namespace, "nginx-app-ingress", createcluster.KubeConfigFile)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("test-ingress"))
			}, "240s", "5s").Should(Succeed())

			var ingressIps []string
			nodes, err := terraform.WorkerNodes(createcluster.KubeConfigFile, false)
			if err != nil {
				fmt.Println("Error retrieving nodes: ", err)
			}
			Eventually(func(g Gomega) {
				ingressIps, err = terraform.FetchIngressIP(namespace, createcluster.KubeConfigFile)
				g.Expect(err).NotTo(HaveOccurred(), "Ingress ip is not returned")
				g.Expect(len(ingressIps)).To(Equal(len(nodes)), "Number of ingress IPs should match the number of nodes")
			}, "240s", "5s").Should(Succeed())

			for _, ip := range ingressIps {
				cmd := "curl -s --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				Eventually(func(g Gomega) {
					res, err := terraform.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-ingress"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Daemonset Preupgrade", func() {
			_, err := terraform.DeployWorkload("daemonset.yaml", createcluster.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			nodes, _ := terraform.WorkerNodes(createcluster.KubeConfigFile, false)
			pods, _ := terraform.Pods(createcluster.KubeConfigFile, false)

			Eventually(func(g Gomega) {
				count := terraform.CountOfStringInSlice("test-daemonset", pods)
				g.Expect(count).Should((Equal(len(nodes))), "Daemonset pod count does not match node count")
			}, "420s", "10s").Should(Succeed())
		})

		It("Verifies DNS Access Preupgrade", func() {
			namespace := "auto-dns"
			_, err := terraform.DeployWorkload("dnsutils.yaml", createcluster.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods dnsutils " + "-n " + namespace + " --kubeconfig=" + createcluster.KubeConfigFile
				res, _ := terraform.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("dnsutils"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl -n " + namespace + " --kubeconfig=" + createcluster.KubeConfigFile + " exec -t dnsutils -- nslookup kubernetes.default"
				res, _ := terraform.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
			}, "420s", "2s").Should(Succeed())
		})
	})

	Context("Upgrade nodes via manual:", func() {
		It("Upgrade nodes", func() {
			var cmdUpgradeVersion string
			if *channel != "" {
				// If a channel is specified, upgrade using that channel
				cmdUpgradeVersion = "sudo curl -sfL https://get.rke2.io | sudo INSTALL_RKE2_VERSION=" + *upgradeVersion + " INSTALL_RKE2_CHANNEL=" + *channel + " sh -"
			} else {
				// If no channel is specified, upgrade using the version specified
				cmdUpgradeVersion = "sudo curl -sfL https://get.rke2.io | sudo INSTALL_RKE2_VERSION=" + *upgradeVersion + " sh -"
			}

			versionRegex := regexp.MustCompile("-rc[0-9]+")
			k8sVersion := versionRegex.ReplaceAllString(*upgradeVersion, "")
			re := regexp.MustCompile("[0-9]+")

			nodes, err := terraform.Nodes(createcluster.KubeConfigFile, false)
			if err != nil {
				fmt.Println("Error retrieving nodes: ", err)
			}

			for _, node := range nodes {
				// Every node will be upgraded and restarted

				Eventually(func(g Gomega) {
					fmt.Println("\nUpgrading ", node.Roles, node.ExternalIP)
					terraform.RunCommandOnNode(cmdUpgradeVersion, node.ExternalIP, createcluster.AwsUser, createcluster.AccessKey)
				}, "120s", "2s").Should(Succeed())

				Eventually(func(g Gomega) {
					fmt.Println("Restarting node: ", node.Roles, node.ExternalIP)
					terraform.RunCommandOnNode("sudo systemctl restart rke2-*", node.ExternalIP, createcluster.AwsUser, createcluster.AccessKey)
				}, "240s", "2s").Should(Succeed())
			}

			//Fetch node status & version PostUpgrade
			Eventually(func(g Gomega) {
				expectedNodeCount := createcluster.NumServers + createcluster.NumWorkers
				nodes, err := terraform.Nodes(createcluster.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).To(Equal(expectedNodeCount), "Number of nodes should match the spec")
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"), "Nodes should all be in Ready state after upgrading")
					g.Expect(node.Version).Should(Equal(k8sVersion), "Nodes should all be upgraded to the specified version")
				}
			}, "900s", "30s").Should(Succeed())

			//Fetch pods status PostUpgrade
			Eventually(func(g Gomega) {
				pods, err := terraform.Pods(createcluster.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
						numRunning := re.FindAllString(pod.Ready, 2)
						g.Expect(numRunning[0]).Should(Equal(numRunning[1]), pod.Name, "should have all containers running")
					}
				}
			}, "600s", "5s").Should(Succeed())
		})
	})

	Context("Postupgrade Validations:", func() {
		It("Verifies ClusterIP Service Postupgrade", func() {
			namespace := "auto-clusterip"
			defer terraform.RemoveWorkload("clusterip.yaml", createcluster.KubeConfigFile)

			res, err := terraform.IsAppRunning(namespace, "nginx-app-clusterip", createcluster.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should((ContainSubstring("test-clusterip")))

			clusterip, port, _ := terraform.FetchClusterIP(createcluster.KubeConfigFile, namespace, "nginx-clusterip-svc")
			cmd := "curl -sL --insecure http://" + clusterip + ":" + port + "/name.html"
			nodeExternalIP := terraform.FetchNodeExternalIP(createcluster.KubeConfigFile)
			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					res, err := terraform.RunCommandOnNode(cmd, ip, createcluster.AwsUser, createcluster.AccessKey)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-clusterip"))
				}, "120s", "10s").Should(Succeed())
			}
		})

		It("Verifies NodePort Service Postupgrade", func() {
			namespace := "auto-nodeport"
			defer terraform.RemoveWorkload("nodeport.yaml", createcluster.KubeConfigFile)

			nodeExternalIP := terraform.FetchNodeExternalIP(createcluster.KubeConfigFile)
			cmd := "kubectl get service -n " + namespace + " nginx-nodeport-svc --kubeconfig=" + createcluster.KubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport, err := terraform.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					res, err := terraform.IsAppRunning(namespace, "nginx-app-nodeport", createcluster.KubeConfigFile)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "120s", "5s").Should(Succeed())

				cmd = "curl -sL --insecure http://" + ip + ":" + nodeport + "/name.html"
				Eventually(func(g Gomega) {
					res, err := terraform.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "120s", "5s").Should(Succeed())
			}
		})

		It("Verifies Ingress Postupgrade", func() {
			namespace := "auto-ingress"
			defer terraform.RemoveWorkload("ingress.yaml", createcluster.KubeConfigFile)

			res, err := terraform.IsAppRunning(namespace, "nginx-app-ingress", createcluster.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("test-ingress"))

			var ingressIps []string
			nodes, err := terraform.WorkerNodes(createcluster.KubeConfigFile, false)
			if err != nil {
				fmt.Println("Error retrieving nodes: ", err)
			}
			Eventually(func(g Gomega) {
				ingressIps, err = terraform.FetchIngressIP(namespace, createcluster.KubeConfigFile)
				g.Expect(err).NotTo(HaveOccurred(), "Ingress ip is not returned")
				g.Expect(len(ingressIps)).To(Equal(len(nodes)), "Number of ingress IPs should match the number of nodes")
			}, "120s", "5s").Should(Succeed())

			for _, ip := range ingressIps {
				cmd := "curl -s --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				Eventually(func(g Gomega) {
					res, err := terraform.RunCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-ingress"))
				}, "120s", "5s").Should(Succeed())
			}
		})

		It("Verifies Daemonset Postupgrade", func() {
			defer terraform.RemoveWorkload("daemonset.yaml", createcluster.KubeConfigFile)
			nodes, _ := terraform.WorkerNodes(createcluster.KubeConfigFile, false)
			pods, _ := terraform.Pods(createcluster.KubeConfigFile, false)
			count := terraform.CountOfStringInSlice("test-daemonset", pods)
			Expect(count).Should((Equal(len(nodes))), "Daemonset pod count does not match node count")
		})

		It("Verifies DNS Access Postupgrade", func() {
			namespace := "auto-dns"
			_, err := terraform.DeployWorkload("dnsutils.yaml", createcluster.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")
			defer terraform.RemoveWorkload("dnsutils.yaml", createcluster.KubeConfigFile)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods dnsutils " + "-n " + namespace + " --kubeconfig=" + createcluster.KubeConfigFile
				res, _ := terraform.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("dnsutils"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "120s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl -n " + namespace + " --kubeconfig=" + createcluster.KubeConfigFile + " exec -t dnsutils -- nslookup kubernetes.default"
				res, _ := terraform.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
			}, "120s", "2s").Should(Succeed())
		})
	})

})

var _ = BeforeEach(func() {
	if *destroy {
		Skip("Cluster is being Deleted")
	}
})

var _ = AfterEach(func() {
	if CurrentSpecReport().Failed() {
		fmt.Printf("\nFAILED! %s\n", CurrentSpecReport().FullText())
	} else {
		fmt.Printf("\nPASSED! %s\n", CurrentSpecReport().FullText())
	}
})
