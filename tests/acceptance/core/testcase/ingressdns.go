package testcase

import (
	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/shared/util"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIngress(deployWorkload bool) {
	var ingressIps []string
	if deployWorkload {
		_, err := util.ManageWorkload("create", "ingress.yaml")
		Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")
	}

	getIngressRunning := "kubectl get pods -n auto-ingress -l k8s-app=nginx-app-ingress  --field-selector=status.phase=Running --kubeconfig="
	err := assert.ValidateOnHost(getIngressRunning+util.KubeConfigFile, util.Running)
	if err != nil {
		GinkgoT().Errorf("Error: %v", err)
	}

	nodes, err := util.Nodes(false)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func(Gomega) bool {
		ingressIps, err = util.FetchIngressIP("auto-ingress")
		if err != nil {
			return false
		}
		if len(ingressIps) != len(nodes) {
			return false
		}
		return true
	}, "400s", "3s").Should(BeTrue())

	for _, ip := range ingressIps {
		if assert.CheckComponentCmdHost("curl -s --header host:foo1.bar.com"+" "+
			"http://"+ip+"/name.html", "test-ingress"); err != nil {
			return
		}
	}
}

func TestDnsAccess(deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "dnsutils.yaml")
		Expect(err).NotTo(HaveOccurred(),
			"dnsutils manifest not deployed", err)
	}

	getDnsUtils := "kubectl get pods -n auto-dns dnsutils --kubeconfig="
	err := assert.ValidateOnHost(getDnsUtils+util.KubeConfigFile, util.Running)
	if err != nil {
		GinkgoT().Errorf("Error: %v", err)
	}

	assert.CheckComponentCmdHost(
		util.ExecDnsUtils+util.KubeConfigFile+" -- nslookup kubernetes.default",
		util.Nslookup,
	)
}
