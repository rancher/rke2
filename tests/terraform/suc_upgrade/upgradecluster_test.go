package upgradecluster

import (
	"flag"
	"fmt"
	"regexp"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rancher/rke2/tests/terraform"
)

var tfVars = flag.String("tfvars", "/tests/terraform/modules/config/local.tfvars", "custom .tfvars file from base project path")
var destroy = flag.Bool("destroy", false, "a bool")
var upgradeVersion = flag.String("upgradeVersion", "", "Version to upgrade the cluster to")
var failed bool

func Test_TFUpgradeClusterValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()

	RunSpecs(t, "Upgrade Cluster Test Suite")
}

var _ = Describe("Upgrade Tests:", func() {
	Context("Build Cluster:", func() {
		It("Starts up with no issues", func() {
			status, err := terraform.BuildCluster(&testing.T{}, *tfVars, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal("cluster created"))
			defer GinkgoRecover()
			fmt.Println("Server Node IPS:", terraform.MasterIPs)
			fmt.Println("Agent Node IPS:", terraform.WorkerIPs)
			terraform.PrintFileContents(terraform.KubeConfigFile)
			Expect(terraform.MasterIPs).ShouldNot(BeEmpty())
			if terraform.NumWorkers > 0 {
				Expect(terraform.WorkerIPs).ShouldNot(BeEmpty())
			} else {
				Expect(terraform.WorkerIPs).Should(BeEmpty())
			}
			Expect(terraform.KubeConfigFile).ShouldNot(BeEmpty())
		})

		It("Checks Node and Pod Status", func() {
			defer func() {
				_, err := terraform.Nodes(terraform.KubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving nodes preupgrade: ", err)
				}
				_, err = terraform.Pods(terraform.KubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving pods preupgrade: ", err)
				}
			}()

			fmt.Printf("\nFetching node status preupgrade\n")
			expectedNodeCount := terraform.NumServers + terraform.NumWorkers
			Eventually(func(g Gomega) {
				nodes, err := terraform.Nodes(terraform.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).To(Equal(expectedNodeCount), "Number of nodes should match the spec")
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"), "Nodes should all be in Ready state")
				}
			}, "420s", "5s").Should(Succeed())

			re := regexp.MustCompile("[0-9]+")
			fmt.Printf("\nFetching pod status preupgrade\n")
			Eventually(func(g Gomega) {
				pods, err := terraform.Pods(terraform.KubeConfigFile, false)
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
			_, err := terraform.DeployWorkload("clusterip.yaml", terraform.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + terraform.KubeConfigFile
				res, err := terraform.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should((ContainSubstring("test-clusterip")))
			}, "240s", "5s").Should(Succeed())

			clusterip, port, _ := terraform.FetchClusterIP(terraform.KubeConfigFile, namespace, "nginx-clusterip-svc")
			cmd := "curl -sL --insecure http://" + clusterip + ":" + port + "/name.html"
			nodeExternalIP := terraform.FetchNodeExternalIP(terraform.KubeConfigFile)
			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					res, err := terraform.RunCommandOnNode(cmd, ip, terraform.AwsUser, terraform.AccessKey)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-clusterip"))
				}, "240s", "10s").Should(Succeed())
			}
		})

		It("Verifies NodePort Service Preupgrade", func() {
			namespace := "auto-nodeport"
			_, err := terraform.DeployWorkload("nodeport.yaml", terraform.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")

			nodeExternalIP := terraform.FetchNodeExternalIP(terraform.KubeConfigFile)
			cmd := "kubectl get service -n " + namespace + " nginx-nodeport-svc --kubeconfig=" + terraform.KubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport, err := terraform.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + terraform.KubeConfigFile
					res, err := terraform.RunCommand(cmd)
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
			_, err := terraform.DeployWorkload("ingress.yaml", terraform.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-ingress --field-selector=status.phase=Running --kubeconfig=" + terraform.KubeConfigFile
				res, err := terraform.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("test-ingress"))
			}, "240s", "5s").Should(Succeed())

			var ingressIps []string
			nodes, err := terraform.WorkerNodes(terraform.KubeConfigFile, false)
			if err != nil {
				fmt.Println("Error retrieving nodes: ", err)
			}
			Eventually(func(g Gomega) {
				ingressIps, err = terraform.FetchIngressIP(namespace, terraform.KubeConfigFile)
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
			_, err := terraform.DeployWorkload("daemonset.yaml", terraform.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")

			nodes, _ := terraform.WorkerNodes(terraform.KubeConfigFile, false)
			pods, _ := terraform.Pods(terraform.KubeConfigFile, false)

			Eventually(func(g Gomega) {
				count := terraform.CountOfStringInSlice("test-daemonset", pods)
				g.Expect(count).Should((Equal(len(nodes))), "Daemonset pod count does not match node count")
			}, "240s", "10s").Should(Succeed())
		})

		It("Verifies DNS Access Preupgrade", func() {
			namespace := "auto-dns"
			_, err := terraform.DeployWorkload("dnsutils.yaml", terraform.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods dnsutils " + "-n " + namespace + " --kubeconfig=" + terraform.KubeConfigFile
				res, _ := terraform.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("dnsutils"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "240s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl -n " + namespace + " --kubeconfig=" + terraform.KubeConfigFile + " exec -t dnsutils -- nslookup kubernetes.default"
				res, _ := terraform.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
			}, "240s", "2s").Should(Succeed())
		})
	})

	Context("Upgrade via SUC:", func() {
		It("Verifies Upgrade", func() {
			namespace := "system-upgrade"
			_, err := terraform.DeployWorkload("suc.yaml", terraform.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "system-upgrade-controller manifest did not deploy successfully")

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods " + "-n " + namespace + " --kubeconfig=" + terraform.KubeConfigFile
				res, _ := terraform.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("system-upgrade-controller"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "120s", "2s").Should(Succeed())

			err = upgradeCluster(*upgradeVersion, terraform.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "failed to upgrade cluster.")

			defer func() {
				_, err := terraform.Nodes(terraform.KubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving nodes postupgrade: ", err)
				}
				_, err = terraform.Pods(terraform.KubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving pods postupgrade: ", err)
				}
			}()
			versionRegex := regexp.MustCompile("-rc[0-9]+")
			k8sVersion := versionRegex.ReplaceAllString(*upgradeVersion, "")
			fmt.Printf("\nFetching node status postupgrade\n")
			expectedNodeCount := terraform.NumServers + terraform.NumWorkers
			Eventually(func(g Gomega) {
				nodes, err := terraform.Nodes(terraform.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).To(Equal(expectedNodeCount), "Number of nodes should match the spec")
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"), "Nodes should all be in Ready state after upgrading")
					g.Expect(node.Version).Should(Equal(k8sVersion), "Nodes should all be upgraded to the specified version")
				}
			}, "900s", "30s").Should(Succeed())

			fmt.Printf("\n Fetching pod status postupgrade\n")
			Eventually(func(g Gomega) {
				pods, err := terraform.Pods(terraform.KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, pod := range pods {
					if strings.Contains(pod.Name, "helm-install") {
						g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
					} else {
						g.Expect(pod.Status).Should(Equal("Running"), pod.Name)
						numRunning := versionRegex.FindAllString(pod.Ready, 2)
						g.Expect(numRunning[0]).Should(Equal(numRunning[1]), pod.Name, "should have all containers running")
					}
				}
			}, "600s", "5s").Should(Succeed())
		})
	})

	Context("Postupgrade Validations:", func() {
		It("Verifies ClusterIP Service Postupgrade", func() {
			namespace := "auto-clusterip"
			defer terraform.RemoveWorkload("clusterip.yaml", terraform.KubeConfigFile)

			cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + terraform.KubeConfigFile
			res, err := terraform.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should((ContainSubstring("test-clusterip")))

			clusterip, port, _ := terraform.FetchClusterIP(terraform.KubeConfigFile, namespace, "nginx-clusterip-svc")
			cmd = "curl -sL --insecure http://" + clusterip + ":" + port + "/name.html"
			nodeExternalIP := terraform.FetchNodeExternalIP(terraform.KubeConfigFile)
			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					res, err := terraform.RunCommandOnNode(cmd, ip, terraform.AwsUser, terraform.AccessKey)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-clusterip"))
				}, "120s", "10s").Should(Succeed())
			}
		})

		It("Verifies NodePort Service Postupgrade", func() {
			namespace := "auto-nodeport"
			defer terraform.RemoveWorkload("nodeport.yaml", terraform.KubeConfigFile)

			nodeExternalIP := terraform.FetchNodeExternalIP(terraform.KubeConfigFile)
			cmd := "kubectl get service -n " + namespace + " nginx-nodeport-svc --kubeconfig=" + terraform.KubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport, err := terraform.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + terraform.KubeConfigFile
					res, err := terraform.RunCommand(cmd)
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
			defer terraform.RemoveWorkload("ingress.yaml", terraform.KubeConfigFile)

			cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-ingress --field-selector=status.phase=Running --kubeconfig=" + terraform.KubeConfigFile
			res, err := terraform.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(res).Should(ContainSubstring("test-ingress"))

			var ingressIps []string
			nodes, err := terraform.WorkerNodes(terraform.KubeConfigFile, false)
			if err != nil {
				fmt.Println("Error retrieving nodes: ", err)
			}
			Eventually(func(g Gomega) {
				ingressIps, err = terraform.FetchIngressIP(namespace, terraform.KubeConfigFile)
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
			defer terraform.RemoveWorkload("daemonset.yaml", terraform.KubeConfigFile)
			nodes, _ := terraform.WorkerNodes(terraform.KubeConfigFile, false)
			pods, _ := terraform.Pods(terraform.KubeConfigFile, false)
			count := terraform.CountOfStringInSlice("test-daemonset", pods)
			Expect(count).Should((Equal(len(nodes))), "Daemonset pod count does not match node count")
		})

		It("Verifies DNS Access Postupgrade", func() {
			namespace := "auto-dns"
			_, err := terraform.DeployWorkload("dnsutils.yaml", terraform.KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")
			defer terraform.RemoveWorkload("dnsutils.yaml", terraform.KubeConfigFile)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods dnsutils " + "-n " + namespace + " --kubeconfig=" + terraform.KubeConfigFile
				res, _ := terraform.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("dnsutils"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "120s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl -n " + namespace + " --kubeconfig=" + terraform.KubeConfigFile + " exec -t dnsutils -- nslookup kubernetes.default"
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
