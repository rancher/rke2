package testcase

import (
	"fmt"
	"strings"

	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/gomega"
)

// TestPodStatus test the status of the pods in the cluster using 2 custom assert functions
func TestPodStatus(
	podAssertRestarts assert.PodAssertFunc,
	podAssertReady assert.PodAssertFunc,
	podAssertStatus assert.PodAssertFunc,
) {
	fmt.Printf("\nFetching pod status\n")

	Eventually(func(g Gomega) {
		pods, err := shared.ParsePods(false)
		g.Expect(err).NotTo(HaveOccurred())

		for _, pod := range pods {
			if strings.Contains(pod.Name, "helm-install") {
				g.Expect(pod.Status).Should(Equal("Completed"), pod.Name)
			} else if strings.Contains(pod.Name, "apply") &&
				strings.Contains(pod.NameSpace, "system-upgrade") {
				g.Expect(pod.Status).Should(SatisfyAny(
					ContainSubstring("Error"),
					Equal("Completed"),
				), pod.Name)
			} else {
				g.Expect(pod.Status).Should(Equal(Running), pod.Name)
				if podAssertRestarts != nil {
					podAssertRestarts(g, pod)
				}
				if podAssertReady != nil {
					podAssertReady(g, pod)
				}
				if podAssertStatus != nil {
					podAssertStatus(g, pod)
				}
			}
		}
	}, "600s", "3s").Should(Succeed())
}

func testCrossNodeServiceRequest(services, ports, expected []string) error {
	var err error
	if len(services) != len(ports) && len(ports) != len(expected) {
		return fmt.Errorf("array parameters must have equal length")
	}

	if len(services) < 2 || len(ports) < 2 || len(expected) < 2 {
		return fmt.Errorf("array parameters must not be less than or equal to 2")
	}

	// Iterating services first to last
	for i := 0; i < len(services); i++ {
		for j := i + 1; j < len(services); j++ {
			cmd := fmt.Sprintf("kubectl exec svc/%s --kubeconfig=%s -- curl -m7 %s:%s",
				services[i], shared.KubeConfigFile, services[j], ports[j])
			Eventually(func() (string, error) {
				return shared.RunCommandHost(cmd)
			}, "120s", "5s").Should(ContainSubstring(expected[j]))
		}
	}

	// Iterating services last to first
	for i := len(services) - 1; i > 0; i-- {
		for j := 1; j <= i; j++ {
			cmd := fmt.Sprintf("kubectl exec svc/%s --kubeconfig=%s -- curl -m7 %s:%s",
				services[i], shared.KubeConfigFile, services[i-j], ports[i-j])
			Eventually(func() (string, error) {
				return shared.RunCommandHost(cmd)
			}, "120s", "5s").Should(ContainSubstring(expected[i-j]))
		}
	}

	return err
}
