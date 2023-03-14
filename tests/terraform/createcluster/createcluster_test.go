package createcluster

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

func Test_TFClusterCreateValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()

	RunSpecs(t, "Create Cluster Test Suite")
}

var _ = Describe("Test:", func() {
	Context("Build Cluster:", func() {
		It("Starts up with no issues", func() {
			status, err := BuildCluster(&testing.T{}, *tfVars, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal("cluster created"))
			defer GinkgoRecover()
			fmt.Println("Server Node IPS:", MasterIPs)
			fmt.Println("Agent Node IPS:", WorkerIPs)
			fmt.Println(KubeConfigFile)
			Expect(MasterIPs).ShouldNot(BeEmpty())
			if NumWorkers > 0 {
				Expect(WorkerIPs).ShouldNot(BeEmpty())
			} else {
				Expect(WorkerIPs).Should(BeEmpty())
			}
			Expect(KubeConfigFile).ShouldNot(BeEmpty())
		})

		It("Checks Node and Pod Status", func() {
			defer func() {
				_, err := terraform.Nodes(KubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving nodes: ", err)
				}
				_, err = terraform.Pods(KubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving pods: ", err)
				}
			}()

			fmt.Printf("\nFetching node status\n")
			expectedNodeCount := NumServers + NumWorkers
			Eventually(func(g Gomega) {
				nodes, err := terraform.Nodes(KubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).To(Equal(expectedNodeCount), "Number of nodes should match the spec")
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"), "Nodes should all be in Ready state")
				}
			}, "420s", "5s").Should(Succeed())

			re := regexp.MustCompile("[0-9]+")
			fmt.Printf("\nFetching pod status\n")
			Eventually(func(g Gomega) {
				pods, err := terraform.Pods(KubeConfigFile, false)
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

		It("Verifies ClusterIP Service", func() {
			namespace := "auto-clusterip"
			_, err := terraform.DeployWorkload("clusterip.yaml", KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")
			defer terraform.RemoveWorkload("clusterip.yaml", KubeConfigFile)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + KubeConfigFile
				res, err := terraform.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("test-clusterip"))
			}, "420s", "5s").Should(Succeed())

			clusterip, port, _ := terraform.FetchClusterIP(KubeConfigFile, namespace, "nginx-clusterip-svc")
			cmd := "curl -sL --insecure http://" + clusterip + ":" + port + "/name.html"
			nodeExternalIP := terraform.FetchNodeExternalIP(KubeConfigFile)
			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					res, err := terraform.RunCommandOnNode(cmd, ip, AwsUser, AccessKey)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-clusterip"))
				}, "420s", "10s").Should(Succeed())
			}
		})

		It("Verifies NodePort Service", func() {
			namespace := "auto-nodeport"
			_, err := terraform.DeployWorkload("nodeport.yaml", KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")
			defer terraform.RemoveWorkload("nodeport.yaml", KubeConfigFile)

			nodeExternalIP := terraform.FetchNodeExternalIP(KubeConfigFile)
			cmd := "kubectl get service -n " + namespace + " nginx-nodeport-svc --kubeconfig=" + KubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport, err := terraform.RunCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + KubeConfigFile
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

		It("Verifies Ingress", func() {
			namespace := "auto-ingress"
			_, err := terraform.DeployWorkload("ingress.yaml", KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")
			defer terraform.RemoveWorkload("ingress.yaml", KubeConfigFile)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-ingress --field-selector=status.phase=Running --kubeconfig=" + KubeConfigFile
				res, err := terraform.RunCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("test-ingress"))
			}, "240s", "5s").Should(Succeed())

			var ingressIps []string
			nodes, err := terraform.WorkerNodes(KubeConfigFile, false)
			if err != nil {
				fmt.Println("Error retrieving nodes: ", err)
			}
			Eventually(func(g Gomega) {
				ingressIps, err = terraform.FetchIngressIP(namespace, KubeConfigFile)
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

		It("Verifies Daemonset", func() {
			_, err := terraform.DeployWorkload("daemonset.yaml", KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")
			defer terraform.RemoveWorkload("daemonset.yaml", KubeConfigFile)

			nodes, _ := terraform.WorkerNodes(KubeConfigFile, false)
			pods, _ := terraform.Pods(KubeConfigFile, false)

			Eventually(func(g Gomega) {
				count := terraform.CountOfStringInSlice("test-daemonset", pods)
				g.Expect(count).Should(Equal(len(nodes)), "Daemonset pod count does not match node count")
			}, "420s", "10s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			namespace := "auto-dns"
			_, err := terraform.DeployWorkload("dnsutils.yaml", KubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")
			defer terraform.RemoveWorkload("dnsutils.yaml", KubeConfigFile)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods dnsutils " + "-n " + namespace + " --kubeconfig=" + KubeConfigFile
				res, _ := terraform.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("dnsutils"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl -n " + namespace + " --kubeconfig=" + KubeConfigFile + " exec -t dnsutils -- nslookup kubernetes.default"
				res, _ := terraform.RunCommand(cmd)
				g.Expect(res).Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
			}, "420s", "2s").Should(Succeed())
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

var _ = AfterSuite(func() {
	if *destroy {
		status, err := BuildCluster(&testing.T{}, *tfVars, *destroy)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("cluster destroyed"))
	}
})
