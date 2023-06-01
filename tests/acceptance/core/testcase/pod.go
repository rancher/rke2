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
		pods, err := shared.Pods(false)
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
