package assert

import (
	"fmt"
	"strings"

	"github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/terraform/shared/util"
)

type PodAssertFunc func(g gomega.Gomega, pod util.Pod)
type NodeAssertFunc func(g gomega.Gomega, node util.Node)

// PodAssertRestarts custom assertion func that asserts that pods are not restarting with no reason
func PodAssertRestarts() PodAssertFunc {
	return func(g gomega.Gomega, pod util.Pod) {
		if strings.Contains(pod.Name, "helm-install") {
			g.Expect(pod.Restarts).Should(gomega.SatisfyAny(gomega.Equal("0"),
				gomega.Equal("1")),
				"helm could be restarted occasionally when cluster started",
				pod.Name)
		}
		g.Expect(pod.Restarts).Should(gomega.Equal("0"),
			"should have all pods running or completed",
			pod.Name)
	}
}

// NodeAssertVersionUpgraded NodeAssertVersion custom assertion func that asserts that node
// is upgraded to the specified version
func NodeAssertVersionUpgraded() NodeAssertFunc {
	return func(g gomega.Gomega, node util.Node) {
		g.Expect(node.Version).Should(gomega.Equal(*util.UpgradeVersion),
			"Nodes should all be upgraded to the specified version", node.Name)
	}
}

func NodeAssertReadyStatus() NodeAssertFunc {
	return func(g gomega.Gomega, node util.Node) {
		g.Expect(node.Status).Should(gomega.Equal("Ready"),
			"Nodes should all be in Ready state")
	}
}

// PodAssertReadyStatus custom assertion func that asserts that the pod is
// with correct numbers of ready containers
func PodAssertReadyStatus() PodAssertFunc {
	return func(g gomega.Gomega, pod util.Pod) {
		g.ExpectWithOffset(1, pod.Ready).To(CheckReadyFields(),
			"should have equal values in n/n format")
	}
}

// CheckReadyFields is a custom matcher that checks
// if the input string is in N/N format and the same qty
func CheckReadyFields() gomega.OmegaMatcher {
	return gomega.WithTransform(func(s string) (bool, error) {
		var a, b int
		n, err := fmt.Sscanf(s, "%d/%d", &a, &b)
		if err != nil || n != 2 {
			return false, fmt.Errorf("failed to parse format: %v", err)
		}
		return a == b, nil
	}, gomega.BeTrue())
}

// CheckComponentCmdHost runs a command on the host and asserts that the value received
// contains the specified substring
func CheckComponentCmdHost(cmd string, asserts ...string) error {
	gomega.Eventually(func(g gomega.Gomega) {
		res, err := util.RunCommandHost(cmd)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		for _, assert := range asserts {
			g.Expect(res).Should(gomega.ContainSubstring(assert))
		}
	}, "420s", "5s").Should(gomega.Succeed())

	return nil
}

// CheckComponentCmdNode runs a command on a node and asserts that the value received
// contains the specified substring
func CheckComponentCmdNode(cmd, ip, assert, sshUser, sshKey string) error {
	gomega.Eventually(func(g gomega.Gomega) {
		res, err := util.RunCommandOnNode(cmd, ip, sshUser, sshKey)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(res).Should(gomega.ContainSubstring(assert))
	}, "420s", "5s").Should(gomega.Succeed())

	return nil
}

// CheckPodStatusRunning asserts that the pod is running with the specified label = app name
func CheckPodStatusRunning(name, namespace, assert string) error {
	gomega.Eventually(func(g gomega.Gomega) {
		cmd := "kubectl get pods -n " + namespace + " -o=name -l k8s-app=" + name +
			" --field-selector=status.phase=Running --kubeconfig=" + util.KubeConfigFile
		res, err := util.RunCommandHost(cmd)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(res).Should(gomega.ContainSubstring(assert))
	}, "420s", "5s").Should(gomega.Succeed())

	return nil
}
