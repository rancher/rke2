package testcase

import (
	"github.com/rancher/rke2/tests/acceptance/core/service/assert"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestServiceClusterIp(deployWorkload bool) {
	if deployWorkload {
		_, err := shared.ManageWorkload("apply", "clusterip.yaml")
		Expect(err).NotTo(HaveOccurred(), "Cluster IP manifest not deployed")
	}

	getClusterIp := "kubectl get pods -n auto-clusterip -l k8s-app=nginx-app-clusterip" +
		" --field-selector=status.phase=Running  --kubeconfig="
	err := assert.ValidateOnHost(getClusterIp+shared.KubeConfigFile, Running)
	if err != nil {
		GinkgoT().Errorf("Error: %v", err)
	}

	clusterip, port, _ := shared.FetchClusterIP("auto-clusterip",
		"nginx-clusterip-svc")
	nodeExternalIP := shared.FetchNodeExternalIP()
	for _, ip := range nodeExternalIP {
		err = assert.ValidateOnNode(ip, "curl -sL --insecure http://"+clusterip+
			":"+port+"/name.html", "test-clusterip")
		if err != nil {
			GinkgoT().Errorf("Error: %v", err)
		}
	}
}

func TestServiceNodePort(deployWorkload bool) {
	if deployWorkload {
		_, err := shared.ManageWorkload("apply", "nodeport.yaml")
		Expect(err).NotTo(HaveOccurred(), "NodePort manifest not deployed")
	}

	nodeExternalIP := shared.FetchNodeExternalIP()
	getNodePortSVC := "kubectl get service -n auto-nodeport nginx-nodeport-svc" +
		" --output jsonpath={.spec.ports[0].nodePort} --kubeconfig="
	nodeport, err := shared.RunCommandHost(getNodePortSVC + shared.KubeConfigFile)
	if err != nil {
		GinkgoT().Errorf("Error: %v", err)
	}

	for _, ip := range nodeExternalIP {
		assert.CheckPodStatusRunning("nginx-app-nodeport",
			"auto-nodeport", "test-nodeport")

		assert.CheckComponentCmdNode("curl -sL --insecure http://"+ip+":"+nodeport+"/name.html",
			"test-nodeport", ip)
		if err != nil {
			GinkgoT().Errorf("Error: %v", err)
		}
	}
}
