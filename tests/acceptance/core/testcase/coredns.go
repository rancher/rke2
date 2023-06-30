package testcase

import (
	"log"

	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/gomega"
)

func TestCoredns(deployWorkload bool) {
	if deployWorkload {
		_, err := shared.ManageWorkload("create", "dnsutils.yaml")
		Expect(err).NotTo(HaveOccurred(),
			"dnsutils manifest not deployed", err)
	}

	_, err := shared.AddHelmRepo("traefik", "https://helm.traefik.io/traefik")
	if err != nil {
		log.Fatalf("failed to add Helm repo: %v", err)
	}

	kubeconfigFlag := " --kubeconfig=" + shared.KubeConfigFile
	fullCmd := shared.JoinCommands("helm list --all-namespaces ", kubeconfigFlag)
	assert.CheckComponentCmdHost(
		fullCmd,
		"rke2-coredns-1.19.402",
	)

	err = assert.ValidateOnHost(ExecDnsUtils+shared.KubeConfigFile+
		" -- nslookup kubernetes.default", Nslookup)
	if err != nil {
		return
	}
}
