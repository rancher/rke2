package testcase

import (
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/gomega"
)

func TestDaemonset(deployWorkload bool) {
	if deployWorkload {
		_, err := shared.ManageWorkload("apply", "daemonset.yaml")
		Expect(err).NotTo(HaveOccurred(),
			"Daemonset manifest not deployed")
	}
	nodes, _ := shared.WorkerNodes(false)
	pods, _ := shared.Pods(false)

	Eventually(func(g Gomega) {
		count := shared.CountOfStringInSlice("test-daemonset", pods)
		g.Expect(count).Should(Equal(len(nodes)),
			"Daemonset pod count does not match node count")
	}, "420s", "5s").Should(Succeed())
}
