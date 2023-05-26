package testcase

import (
	"github.com/rancher/rke2/tests/acceptance/shared/util"

	. "github.com/onsi/gomega"
)

func TestDaemonset(deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "daemonset.yaml")
		Expect(err).NotTo(HaveOccurred(),
			"Daemonset manifest not deployed")
	}
	nodes, _ := util.WorkerNodes(false)
	pods, _ := util.Pods(false)

	Eventually(func(g Gomega) {
		count := util.CountOfStringInSlice("test-daemonset", pods)
		g.Expect(count).Should(Equal(len(nodes)),
			"Daemonset pod count does not match node count")
	}, "420s", "5s").Should(Succeed())
}
