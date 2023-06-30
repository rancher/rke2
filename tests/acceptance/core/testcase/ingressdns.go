package testcase

import (
	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var Running = "Running"
var ExecDnsUtils = "kubectl exec -n auto-dns -t dnsutils --kubeconfig="
var Nslookup = "kubernetes.default.svc.cluster.local"

func TestIngress(deployWorkload bool) {
	var ingressIps []string
	if deployWorkload {
		_, err := shared.ManageWorkload("create", "ingress.yaml")
		Expect(err).NotTo(HaveOccurred(), "Ingress manifest not deployed")
	}

	getIngressRunning := "kubectl get pods -n auto-ingress -l k8s-app=nginx-app-ingress  " +
		"--field-selector=status.phase=Running --kubeconfig="
	err := assert.ValidateOnHost(getIngressRunning+shared.KubeConfigFile, Running)
	if err != nil {
		GinkgoT().Errorf("Error: %v", err)
	}

	nodes, err := shared.Nodes(false)
	Expect(err).NotTo(HaveOccurred())

	Eventually(func(Gomega) bool {
		ingressIps, err = shared.FetchIngressIP("auto-ingress")
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
		_, err := shared.ManageWorkload("create", "dnsutils.yaml")
		Expect(err).NotTo(HaveOccurred(),
			"dnsutils manifest not deployed", err)
	}

	getDnsUtils := "kubectl get pods -n auto-dns dnsutils --kubeconfig="
	err := assert.ValidateOnHost(getDnsUtils+shared.KubeConfigFile, Running)
	if err != nil {
		GinkgoT().Errorf("Error: %v", err)
	}

	assert.CheckComponentCmdHost(
		ExecDnsUtils+shared.KubeConfigFile+" -- nslookup kubernetes.default",
		Nslookup,
	)
}
