package testfunctions

import (
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/terraform/core/assert"
	"github.com/rancher/rke2/tests/terraform/shared/util"
)

func TestTFIngress(t ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "ingress.yaml")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Ingress manifest not deployed")
	}
	err := assert.CheckComponentCmdHost("kubectl get pods -n auto-ingress -o=name -l k8s-app=nginx-app-ingress"+
		" --field-selector=status.phase=Running --kubeconfig="+util.KubeConfigFile, "ingress")
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Error:", err)

	var ingressIps []string
	nodes, err := util.WorkerNodes(false)
	if err != nil {
		fmt.Println("Error retrieving nodes: ", err)
	}

	gomega.Eventually(func(g gomega.Gomega) {
		ingressIps, err = util.FetchIngressIP("auto-ingress")
		g.Expect(err).NotTo(gomega.HaveOccurred(), "Ingress ip is not returned")
		g.Expect(len(ingressIps)).To(gomega.Equal(len(nodes)),
			"Number of ingress IPs should match the number of nodes")
	}, "240s", "5s").Should(gomega.Succeed())

	for _, ip := range ingressIps {
		err := assert.CheckComponentCmdHost("curl -s --header host:foo1.bar.com"+" "+
			"http://"+ip+"/name.html", "test-ingress")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Error:", err)
	}
}

func TestDnsAccess(t ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "dnsutils.yaml")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			"dnsutils manifest not deployed", err)
	}
	err := assert.CheckComponentCmdHost("kubectl get pods -n auto-dns dnsutils --kubeconfig="+
		util.KubeConfigFile, "dnsutils")
	if err != nil {
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Error:", err)

		err = assert.CheckComponentCmdHost("kubectl -n auto-dns --kubeconfig="+
			util.KubeConfigFile+" exec -t dnsutils -- nslookup kubernetes.default",
			"kubernetes.default.svc.cluster.local")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Error:", err)
	}
}
