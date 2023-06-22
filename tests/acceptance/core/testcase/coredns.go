package testcase

import (
	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/gomega"
)

var ExecDnsUtils = "kubectl exec -n auto-dns -t dnsutils --kubeconfig="
var Nslookup = "kubernetes.default.svc.cluster.local"

func TestCoredns(deployWorkload bool) {
	if deployWorkload {
		_, err := shared.ManageWorkload("apply", "dnsutils.yaml")
		Expect(err).NotTo(HaveOccurred(),
			"dnsutils manifest not deployed", err)
	}
	_, err := shared.AddHelmRepo("traefik", "https://helm.traefik.io/traefik")
	if err != nil {
		return
	}

	err = assert.ValidateOnHost(ExecDnsUtils+shared.KubeConfigFile+
		" -- nslookup kubernetes.default", Nslookup)
	if err != nil {
		return
	}

	ips := shared.FetchNodeExternalIP()
	for _, ip := range ips {
		err = assert.ValidateOnHost(
			ip,
			"helm list --all-namespaces | grep rke2-coredns",
			"rke2-coredns-1.19.402",
		)
		if err != nil {
			return
		}
	}
}
