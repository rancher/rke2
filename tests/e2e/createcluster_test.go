package e2e

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	"strings"
	"testing"
)

func Test_E2ERKE2ClusterCreateValidation(t *testing.T) {
	junitReporter := reporters.NewJUnitReporter(fmt.Sprintf("/config/" + *resourceName + ".xml"))
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Test Suite", []Reporter{junitReporter})
}

var _ = Describe("Test:", func() {
	Context("Build Cluster:", func() {
		Context("Cluster Configuration: OS: "+*nodeOs, func() {
			kubeconfig, masterIPs, workerIPs = BuildCluster(*nodeOs, *installMode, *resourceName, &testing.T{}, *destroy)
			defer GinkgoRecover()
			if *destroy {
				fmt.Printf("\nCluster is being Deleted\n")
				return
			}
			fmt.Printf("\nIPs:\n")
			fmt.Println("Master Node IPS:", masterIPs)
			fmt.Println("Worker Node IPS:", workerIPs)

			fmt.Printf("\nFetching node status\n")
			nodes := ParseNode(kubeconfig, true)
			for _, config := range nodes {
				Expect(config.Status).Should(Equal("Ready"), func() string { return config.Name })
			}

			fmt.Printf("\nFetching Pods status\n")
			pods := ParsePod(kubeconfig, true)
			for _, pod := range pods {
				if strings.Contains(pod.Name, "helm-install") {
					Expect(pod.Status).Should(Equal("Completed"), func() string { return pod.Name })
				} else {
					Expect(pod.Status).Should(Equal("Running"), func() string { return pod.Name })
				}
			}
			kubeconfig = kubeconfig + "_kubeconfig"
			fmt.Printf("export KUBECONFIG=%s\n", kubeconfig) //TODO usage?
		})
	})

	Context("Validate Rebooting nodes", func() {
		if *destroy {
			return
		}
		defer GinkgoRecover()
		nodeExternalIP := FetchNodeExternalIP(kubeconfig)
		for _, ip := range nodeExternalIP {
			fmt.Println("\nRebooting node: ", ip)
			cmd := "ssh -i " + *sshkey + " -o \"StrictHostKeyChecking no\" " + *sshuser + "@" + ip + " sudo reboot"
			_, _ = RunCommand(cmd)
			time.Sleep(3 * time.Minute)

			fmt.Println("\nNode and Pod Status after rebooting node: ", ip)
			nodes := ParseNode(kubeconfig, true)
			for _, config := range nodes {
				Expect(config.Status).Should(Equal("Ready"), func() string { return config.Name })
			}
			pods := ParsePod(kubeconfig, true)
			for _, pod := range pods {
				if strings.Contains(pod.Name, "helm-install") {
					Expect(pod.Status).Should(Equal("Completed"), func() string { return pod.Name })
				} else {
					Expect(pod.Status).Should(Equal("Running"), func() string { return pod.Name })
				}
			}
		}
	})
})

var _ = AfterSuite(func() {
	kubeconfig, masterIPs, workerIPs = BuildCluster(*nodeOs, *installMode, *resourceName, &testing.T{}, true)
})
