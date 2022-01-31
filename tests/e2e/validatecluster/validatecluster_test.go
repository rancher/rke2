package validatecluster

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

// Valid nodeOS: generic/ubuntu2004, opensuse/Leap-15.3.x86_64
var nodeOS = flag.String("nodeOS", "generic/ubuntu2004", "VM operating system")
var serverCount = flag.Int("serverCount", 1, "number of server nodes")
var agentCount = flag.Int("agentCount", 1, "number of agent nodes")

// Valid format: RELEASE_VERSION=v1.22.6+rke2r1 or nil for latest commit from master
var installType = flag.String("installType", "", "a string")

func Test_E2EClusterValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Create Cluster Test Suite")
}

var (
	kubeConfigFile  string
	serverNodeNames []string
	agentNodeNames  []string
)

var _ = Describe("Verify Create", func() {
	Context("Cluster :", func() {
		It("Starts up with no issues", func() {
			var err error
			serverNodeNames, agentNodeNames, err = e2e.CreateCluster(*nodeOS, *serverCount, *agentCount, *installType)
			Expect(err).NotTo(HaveOccurred())
			fmt.Println("CLUSTER CONFIG")
			fmt.Println("OS:", *nodeOS)
			fmt.Println("Server Nodes:", serverNodeNames)
			fmt.Println("Agent Nodes:", agentNodeNames)
			kubeConfigFile, err = e2e.GenKubeConfigFile(serverNodeNames[0])
			Expect(err).NotTo(HaveOccurred())
		})

		It("Checks Node and Pod Status", func() {
			fmt.Printf("\nFetching node status\n")
			Eventually(func(g Gomega) {
				nodes, err := e2e.ParseNodes(kubeConfigFile, false)
				g.Expect(err).NotTo(HaveOccurred())
				for _, node := range nodes {
					g.Expect(node.Status).Should(Equal("Ready"))
				}
			}, "420s", "5s").Should(Succeed())
			_, err := e2e.ParseNodes(kubeConfigFile, true)
			Expect(err).NotTo(HaveOccurred())

			fmt.Printf("\nFetching Pods status\n")
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
			_, err = e2e.ParsePods(kubeConfigFile, true)
			Expect(err).NotTo(HaveOccurred())
		})
		It("Verifies ClusterIP Service", func() {
			_, err := e2e.DeployWorkload("clusterip.yaml", kubeConfigFile)
			if err != nil {
				fmt.Println("Cluster IP manifest not deployed", err)
			}
			Eventually(func(g Gomega) {
				cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-clusterip --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
				res, _ := e2e.RunCommand(cmd)
				g.Expect(res).Should((ContainSubstring("test-clusterip")))
			}, "240s", "5s").Should(Succeed())

			clusterip, _ := e2e.FetchClusterIP(kubeConfigFile, "nginx-clusterip-svc")
			cmd := "\"curl -L --insecure http://" + clusterip + "/name.html\""
			fmt.Println(cmd)
			for _, element := range serverNodeNames {
				nodeName := strings.TrimSpace(element)
				if nodeName == "" {
					continue
				}
				res, _ := e2e.RunCmdOnNode(cmd, nodeName)
				fmt.Println(res)
				Eventually(res).Should(ContainSubstring("test-clusterip"))
			}
		})

		// It("Verifies NodePort Service", func() {
		// 	_, err := e2e.DeployWorkload("nodeport.yaml", kubeConfigFile)
		// 	if err != nil {
		// 		fmt.Println("NodePort manifest not deployed", err)
		// 	}
		// 	for _, element := range serverNodeNames {
		// 		nodeName := strings.TrimSpace(element)
		// 		if nodeName == "" {
		// 			continue
		// 		}
		// 		node_external_ip, _ := e2e.FetchNodeExternalIP(nodeName)
		// 		cmd := "kubectl get service nginx-nodeport-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\""
		// 		nodeport, _ := e2e.RunCommand(cmd)
		// 		cmd = "curl -L --insecure http://" + node_external_ip + ":" + nodeport + "/name.html"
		// 		fmt.Println(cmd)
		// 		res, _ := e2e.RunCommand(cmd)
		// 		fmt.Println(res)
		// 		Eventually(func(g Gomega) {
		// 			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-nodeport --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
		// 			res, _ := e2e.RunCommand(cmd)
		// 			g.Expect(res).Should(ContainSubstring("test-nodeport"), "nodeport pod was not created")
		// 		}, "240s", "5s").Should(Succeed())

		// 	}
		// })

		// It("Verifies LoadBalancer Service", func() {
		// 	_, err := e2e.DeployWorkload("loadbalancer.yaml", kubeConfigFile)
		// 	if err != nil {
		// 		fmt.Println(err, "loadbalancer manifest not deployed")
		// 	}
		// 	for _, element := range serverNodeNames {
		// 		nodeName := strings.TrimSpace(element)
		// 		if nodeName == "" {
		// 			continue
		// 		}
		// 		ip, _ := e2e.FetchNodeExternalIP(nodeName)
		// 		cmd := "kubectl get service nginx-loadbalancer-svc --kubeconfig=" + kubeConfigFile + " --output jsonpath=\"{.spec.ports[0].port}\""
		// 		port, _ := e2e.RunCommand(cmd)
		// 		cmd = "curl -L --insecure http://" + ip + ":" + port + "/name.html"
		// 		fmt.Println(cmd)
		// 		res, _ := e2e.RunCommand(cmd)
		// 		fmt.Println(res)

		// 		Eventually(res).Should(ContainSubstring("test-loadbalancer"))
		// 		Eventually(func(g Gomega) {
		// 			cmd := "kubectl get pods -o=name -l k8s-app=nginx-app-loadbalancer --field-selector=status.phase=Running --kubeconfig=" + kubeConfigFile
		// 			res, _ := e2e.RunCommand(cmd)
		// 			g.Expect(res).Should(ContainSubstring("test-loadbalancer"))
		// 		}, "240s", "5s").Should(Succeed())
		// 	}
		// })
		// It("Verifies Ingress", func() {
		// 	_, err := e2e.DeployWorkload("ingress.yaml", kubeConfigFile)
		// 	if err != nil {
		// 		fmt.Println(err, "ingress manifest not deployed")
		// 	}
		// 	for _, element := range serverNodeNames {
		// 		nodeName := strings.TrimSpace(element)
		// 		if nodeName == "" {
		// 			continue
		// 		}
		// 		ip, _ := e2e.FetchNodeExternalIP(nodeName)
		// 		cmd := "curl  --header host:foo1.bar.com" + " http://" + ip + "/name.html"
		// 		fmt.Println(cmd)

		// 		Eventually(func(g Gomega) {
		// 			res, _ := e2e.RunCommand(cmd)
		// 			fmt.Println(res)
		// 			g.Expect(res).Should(ContainSubstring("test-ingress"))
		// 		}, "240s", "5s").Should(Succeed())
		// 	}
		// })

		// It("Verifies Daemonset", func() {
		// 	_, err := e2e.DeployWorkload("daemonset.yaml", kubeConfigFile)
		// 	if err != nil {
		// 		fmt.Println(err, "daemonset manifest not deployed")
		// 	}
		// 	nodes, _ := e2e.ParseNodes(kubeConfigFile, false)
		// 	pods, _ := e2e.ParsePods(kubeConfigFile, false)

		// 	Eventually(func(g Gomega) {
		// 		count := e2e.CountOfStringInSlice("test-daemonset", pods)
		// 		fmt.Println("POD COUNT")
		// 		fmt.Println(count)
		// 		fmt.Println("NODE COUNT")
		// 		fmt.Println(len(nodes))
		// 		g.Expect(len(nodes)).Should((Equal(count)), "Daemonset pod count does not match node count")
		// 	}, "240s", "10s").Should(Succeed())
		// })

		// It("Verifies dns access", func() {
		// 	_, err := e2e.DeployWorkload("dnsutils.yaml", kubeConfigFile)
		// 	if err != nil {
		// 		fmt.Println(err, "dnsutils manifest not deployed")
		// 	}

		// 	Eventually(func(g Gomega) {
		// 		cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec -i -t dnsutils -- nslookup kubernetes.default"
		// 		fmt.Println(cmd)
		// 		res, _ := e2e.RunCommand(cmd)
		// 		fmt.Println(res)
		// 		g.Expect(res).Should(ContainSubstring("kubernetes.default.svc.cluster.local"))
		// 	}, "240s", "2s").Should(Succeed())
		// })

		// It("Verify Local Path Provisioner storage ", func() {
		// 	_, err := e2e.DeployWorkload("local-path-provisioner.yaml", kubeConfigFile)
		// 	if err != nil {
		// 		fmt.Println(err, "local-path-provisioner manifest not deployed")
		// 	}

		// 	Eventually(func(g Gomega) {
		// 		cmd := "kubectl get pvc local-path-pvc --kubeconfig=" + kubeConfigFile
		// 		res, _ := e2e.RunCommand(cmd)
		// 		fmt.Println(res)
		// 		g.Expect(res).Should(ContainSubstring("local-path-pvc"))
		// 		g.Expect(res).Should(ContainSubstring("Bound"))
		// 	}, "240s", "2s").Should(Succeed())

		// 	Eventually(func(g Gomega) {
		// 		cmd := "kubectl get pod volume-test --kubeconfig=" + kubeConfigFile
		// 		res, _ := e2e.RunCommand(cmd)
		// 		fmt.Println(res)

		// 		g.Expect(res).Should(ContainSubstring("volume-test"))
		// 		g.Expect(res).Should(ContainSubstring("Running"))
		// 	}, "420s", "2s").Should(Succeed())

		// 	cmd := "kubectl --kubeconfig=" + kubeConfigFile + " exec volume-test -- sh -c 'echo local-path-test > /data/test'"
		// 	_, _ = e2e.RunCommand(cmd)
		// 	fmt.Println("Data stored in pvc: local-path-test")

		// 	cmd = "kubectl delete pod volume-test --kubeconfig=" + kubeConfigFile
		// 	res, _ := e2e.RunCommand(cmd)
		// 	fmt.Println(res)

		// 	_, err = e2e.DeployWorkload("local-path-provisioner.yaml", kubeConfigFile)
		// 	if err != nil {
		// 		fmt.Println(err, "local-path-provisioner manifest not deployed")
		// 	}

		// 	Eventually(func(g Gomega) {

		// 		cmd = "kubectl exec volume-test cat /data/test --kubeconfig=" + kubeConfigFile
		// 		res, _ = e2e.RunCommand(cmd)
		// 		fmt.Println("Data after re-creation", res)
		// 		g.Expect(res).Should(ContainSubstring("local-path-test"))
		// 	}, "180s", "2s").Should(Succeed())
		// })

	})
})

var failed = false
var _ = AfterEach(func() {
	failed = failed || CurrentGinkgoTestDescription().Failed
})

var _ = AfterSuite(func() {
	if failed {
		fmt.Println("FAILED!")
	} else {
		Expect(e2e.DestroyCluster()).To(Succeed())
		Expect(os.Remove(kubeConfigFile)).To(Succeed())
	}
})
