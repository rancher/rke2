package testfunctions

import (
	"fmt"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/terraform/core/assert"
	"github.com/rancher/rke2/tests/terraform/shared/util"
)

func TestTFServiceClusterIp(t ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "clusterip.yaml")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "Cluster IP manifest not deployed")
	}

	err := assert.CheckPodStatusRunning("nginx-app-clusterip",
		"auto-clusterip", "test-clusterip")
	gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("Pod not running: %s", err))

	clusterip, port, _ := util.FetchClusterIP("auto-clusterip",
		"nginx-clusterip-svc")

	nodeExternalIP := util.FetchNodeExternalIP()
	for _, ip := range nodeExternalIP {
		err := assert.CheckComponentCmdNode("curl -sL --insecure http://"+clusterip+
			":"+port+"/name.html", ip, "test-clusterip", util.AwsUser, util.AccessKey)
		gomega.Expect(err).NotTo(gomega.HaveOccurred(),
			fmt.Sprintf("error: %s", err))
	}
}

func TestTFServiceNodePort(t ginkgo.GinkgoTestingT, deployWorkload bool) {
	if deployWorkload {
		_, err := util.ManageWorkload("create", "nodeport.yaml")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), "NodePort manifest not deployed")
	}

	nodeport, err := util.RunCommandHost("kubectl get service -n auto-nodeport nginx-nodeport-svc --kubeconfig=" +
		util.KubeConfigFile + " --output jsonpath=\"{.spec.ports[0].nodePort}\"")
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	nodeExternalIP := util.FetchNodeExternalIP()
	for _, ip := range nodeExternalIP {
		err := assert.CheckPodStatusRunning("nginx-app-nodeport",
			"auto-nodeport", "test-nodeport")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("Error: %s", err))

		err = assert.CheckComponentCmdHost("curl -sL --insecure http://"+ip+":"+nodeport+"/name.html",
			"test-nodeport")
		gomega.Expect(err).NotTo(gomega.HaveOccurred(), fmt.Sprintf("Error: %s", err))
	}
}
