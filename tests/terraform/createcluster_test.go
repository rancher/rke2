package terraform

import (
	"flag"
	"fmt"
	"regexp"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var tfVars = flag.String("tfvars", "/tests/terraform/modules/config/local.tfvars", "custom .tfvars file from base project path")
var destroy = flag.Bool("destroy", false, "a bool")

var failed bool

func Test_TFClusterCreateValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	flag.Parse()

	RunSpecs(t, "Create Cluster Test Suite")
}

var _ = Describe("Test:", func() {
	Context("Build Cluster:", func() {
		It("Starts up with no issues", func() {
			status, err := buildCluster(&testing.T{}, *tfVars, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(status).To(Equal("cluster created"))
			defer GinkgoRecover()
			fmt.Println("Server Node IPS:", masterIPs)
			fmt.Println("Agent Node IPS:", workerIPs)
			printFileContents(kubeConfigFile)
			Expect(masterIPs).ShouldNot(BeEmpty())
			if numWorkers > 0 {
				Expect(workerIPs).ShouldNot(BeEmpty())
			} else {
				Expect(workerIPs).Should(BeEmpty())
			}
			Expect(kubeConfigFile).ShouldNot(BeEmpty())
		})

		It("Checks Node and Pod Status", func() {
			defer func() {
				_, err := nodes(kubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving nodes: ", err)
				}
				_, err = pods(kubeConfigFile, true)
				if err != nil {
					fmt.Println("Error retrieving pods: ", err)
				}
			}()

			fmt.Printf("\nFetching node status\n")
			expectedNodeCount := numServers + numWorkers
			Eventually(func(g Gomega) {
				nodes, err := nodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(nodes)).To(Equal(expectedNodeCount), "Number of nodes should match the spec")
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"), "Nodes should all be in Ready state")
				}
			}, "420s", "5s").Should(Succeed())

			re := regexp.MustCompile("[0-9]+")
			fmt.Printf("\nFetching pod status\n")
			Eventually(func(g Gomega) {
				pods, err := pods(kubeConfigFile, false)
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
			_, err := deployWorkload("clusterip.yaml", kubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")
			defer removeWorkload("clusterip.yaml", kubeConfigFile)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				res, err := runCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should((ContainSubstring("test-clusterip")))
			}, "420s", "5s").Should(Succeed())

			clusterip, port, _ := fetchClusterIP(kubeConfigFile, namespace, "nginx-clusterip-svc")
			cmd := "curl -sL --insecure http://" + clusterip + ":" + port + "/name.html"
			nodeExternalIP := fetchNodeExternalIP(kubeConfigFile)
			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					res, err := runCmdOnNode(cmd, ip, awsUser, accessKey)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-clusterip"))
				}, "420s", "10s").Should(Succeed())
			}
		})

		It("Verifies NodePort Service", func() {
			namespace := "auto-nodeport"
			_, err := deployWorkload("nodeport.yaml", kubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")
			defer removeWorkload("nodeport.yaml", kubeConfigFile)

			nodeExternalIP := fetchNodeExternalIP(kubeConfigFile)
			cmd := "kubectl get service -n " + namespace + " nginx-nodeport-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
			nodeport, err := runCommand(cmd)
			Expect(err).NotTo(HaveOccurred())

			for _, ip := range nodeExternalIP {
				Eventually(func(g Gomega) {
					cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
					res, err := runCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "240s", "5s").Should(Succeed())

				cmd = "curl -sL --insecure http://" + ip + ":" + nodeport + "/name.html"
				Eventually(func(g Gomega) {
					res, err := runCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-nodeport"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Ingress", func() {
			namespace := "auto-ingress"
			_, err := deployWorkload("ingress.yaml", kubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")
			defer removeWorkload("ingress.yaml", kubeConfigFile)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=nginx-app-ingress --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				res, err := runCommand(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).Should(ContainSubstring("test-ingress"))
			}, "240s", "5s").Should(Succeed())

			var ingressIps []string
			nodes, err := workerNodes(kubeConfigFile, false)
			if err != nil {
				fmt.Println("Error retrieving nodes: ", err)
			}
			Eventually(func(g Gomega) {
				ingressIps, err = fetchIngressIP(namespace, kubeConfigFile)
				g.Expect(err).NotTo(HaveOccurred(), "Ingress ip is not returned")
				g.Expect(len(ingressIps)).To(Equal(len(nodes)), "Number of ingress IPs should match the number of nodes")
			}, "240s", "5s").Should(Succeed())

			for _, ip := range ingressIps {
				cmd := "curl -s --header host:foo1.bar.com" + " http://" + ip + "/name.html"
				Eventually(func(g Gomega) {
					res, err := runCommand(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(res).Should(ContainSubstring("test-ingress"))
				}, "240s", "5s").Should(Succeed())
			}
		})

		It("Verifies Daemonset", func() {
			_, err := deployWorkload("daemonset.yaml", kubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "Daemonset manifest not deployed")
			defer removeWorkload("daemonset.yaml", kubeConfigFile)

			nodes, _ := workerNodes(kubeConfigFile, false)
			pods, _ := pods(kubeConfigFile, false)

			Eventually(func(g Gomega) {
				count := countOfStringInSlice("test-daemonset", pods)
				g.Expect(count).Should((Equal(len(nodes))), "Daemonset pod count does not match node count")
			}, "420s", "10s").Should(Succeed())
		})

		It("Verifies dns access", func() {
			namespace := "auto-dns"
			_, err := deployWorkload("dnsutils.yaml", kubeConfigFile)
			Expect(err).NotTo(HaveOccurred(), "dnsutils manifest not deployed")
			defer removeWorkload("dnsutils.yaml", kubeConfigFile)

			Eventually(func(g Gomega) {
				cmd := "kubectl get pods dnsutils " + "-n " + namespace + " --kubeconfig=" + kubeConfigFile
				res, _ := runCommand(cmd)
				g.Expect(res).Should(ContainSubstring("dnsutils"))
				g.Expect(res).Should(ContainSubstring("Running"))
			}, "420s", "2s").Should(Succeed())

			Eventually(func(g Gomega) {
				cmd := "kubectl -n " + namespace + " --kubeconfig=" + kubeConfigFile + " exec -t dnsutils -- nslookup kubernetes.default"
				res, _ := runCommand(cmd)
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

var _ = AfterSuite(func() {
	if *destroy {
		status, err := buildCluster(&testing.T{}, *tfVars, *destroy)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal("cluster destroyed"))
	}
})
