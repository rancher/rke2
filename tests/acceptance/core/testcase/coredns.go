package testcase

import (
	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/shared/util"

	. "github.com/onsi/gomega"
)

func TestCoredns(deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "dnsutils.yaml")
		Expect(err).NotTo(HaveOccurred(),
			"dnsutils manifest not deployed", err)
	}
	_, err := util.AddHelmRepo("traefik", "https://helm.traefik.io/traefik")
	if err != nil {
		return
	}

	err = assert.ValidateOnHost(util.ExecDnsUtils+util.KubeConfigFile+
		" -- nslookup kubernetes.default", util.Nslookup)
	if err != nil {
		return
	}

	ips := util.FetchNodeExternalIP()
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
