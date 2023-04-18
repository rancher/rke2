package testfunctions

import (
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/terraform/shared/util"
)

func TestDaemonset(t ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "daemonset.yaml")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"Daemonset manifest not deployed")
	}

	nodes, _ := util.WorkerNodes(false)
	pods, _ := util.Pods(false)

	gomega.Eventually(func(g gomega.Gomega) {
		count := util.CountOfStringInSlice("test-daemonset", pods)
		g.Expect(count).Should(gomega.Equal(len(nodes)),
			"Daemonset pod count does not match node count")
	}, "420s", "10s").Should(gomega.Succeed())
}
