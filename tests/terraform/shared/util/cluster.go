package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/onsi/gomega"
	"github.com/rancher/rke2/tests/terraform/core/rke2error"
)

type Node struct {
	Name       string
	Status     string
	Roles      string
	Version    string
	InternalIP string
	ExternalIP string
}

type Pod struct {
	NameSpace string
	Name      string
	Ready     string
	Status    string
	Restarts  string
	NodeIP    string
	Node      string
}

func ManageWorkload(action, workload string) (string, error) {
	if action != "create" && action != "delete" {
		return "", fmt.Errorf("invalid action: %s. Must be 'create' or 'delete'", action)
	}
	resourceDir := Basepath() + "/shared/workloads/"

	files, err := os.ReadDir(resourceDir)
	if err != nil {
		err = fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)

		return "", err
	}

	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			var cmd string

			if action == "create" {
				fmt.Println("\nDeploying", workload)
				cmd = "kubectl apply -f " + filename + " --kubeconfig=" + KubeConfigFile
			} else {
				fmt.Println("\nRemoving", workload)
				cmd = "kubectl delete -f " + filename + " --kubeconfig=" + KubeConfigFile
			}
			res, err := RunCommandHost(cmd)

			if action == "delete" {
				gomega.Eventually(func(g gomega.Gomega) {
					isDeleted, err := IsWorkloadDeleted(workload)
					g.Expect(err).NotTo(gomega.HaveOccurred())
					g.Expect(isDeleted).To(gomega.BeTrue(),
						"Workload should be deleted")
				}, "60s", "5s").Should(gomega.Succeed())
			}

			return res, err
		}
	}

	return "", nil
}

func IsWorkloadDeleted(workload string) (bool, error) {
	cmd := fmt.Sprintf("kubectl get all -A --kubeconfig=%s", KubeConfigFile)
	res, err := RunCommandHost(cmd)
	if err != nil {
		return false, err
	}

	return !strings.Contains(res, workload), nil
}

// Nodes returns the list of nodes in the cluster and parses the output with parseNodes
func Nodes(print bool) ([]Node, error) {
	cmd := "kubectl get nodes --no-headers -o wide --kubeconfig=" + KubeConfigFile
	return parseNodes(print, cmd)
}

// WorkerNodes returns the list of worker nodes in the cluster
func WorkerNodes(print bool) ([]Node, error) {
	cmd := "kubectl get node -o jsonpath='{range .items[*]}{@.metadata.name} " +
		"{@.status.conditions[-1].type} <not retrieved> <not retrieved> " +
		"{@.status.nodeInfo.kubeletVersion} " +
		"{@.status.addresses[?(@.type==\"InternalIP\")].address} " +
		"{@.status.addresses[?(@.type==\"ExternalIP\")].address} " +
		"{@.spec.taints[*].effect}{\"\\n\"}{end}' " +
		"--kubeconfig=" + KubeConfigFile + " | grep -v NoSchedule | grep -v NoExecute"

	return parseNodes(print, cmd)
}

// Pods returns the list of pods in the cluster and parses the output with parsePods
func Pods(print bool) ([]Pod, error) {
	cmd := "kubectl get pods -o wide --no-headers -A --kubeconfig=" + KubeConfigFile
	return parsePods(print, cmd)
}

// FetchClusterIP returns the cluster IP and port of the service
func FetchClusterIP(
	namespace string,
	servicename string,
) (string, string, error) {
	ipCmd := "kubectl get svc " + servicename + " -n " + namespace +
		" -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + KubeConfigFile
	ip, err := RunCommandHost(ipCmd)
	if err != nil {
		return "", "", err
	}
	portCmd := "kubectl get svc " + servicename + " -n " + namespace +
		" -o jsonpath='{.spec.ports[0].port}' --kubeconfig=" + KubeConfigFile
	port, err := RunCommandHost(portCmd)
	if err != nil {
		return "", "", err
	}

	return ip, port, err
}

// FetchNodeExternalIP returns the external IP of the nodes
func FetchNodeExternalIP() []string {
	cmd := "kubectl get node --output=jsonpath='{range .items[*]} " +
		"{ .status.addresses[?(@.type==\"ExternalIP\")].address}' --kubeconfig=" + KubeConfigFile
	time.Sleep(10 * time.Second)
	res, _ := RunCommandHost(cmd)
	nodeExternalIP := strings.Trim(res, " ")
	nodeExternalIPs := strings.Split(nodeExternalIP, " ")

	return nodeExternalIPs
}

// RestartCluster restarts the k3s service on each node given
// server can be server, cp or etcd
func RestartCluster(clusterType string) error {
	nodeExternalIps := FetchNodeExternalIP()
	if clusterType == "server" {
		for _, ip := range nodeExternalIps {
			if _, err := RunCommandOnNode("sudo systemctl restart rke2-server", ip,
				AwsUser, AccessKey); err != nil {
				return err
			}
		}
	}
	for _, ip := range nodeExternalIps {
		if _, err := RunCommandOnNode("sudo systemctl restart rke2-agent", ip,
			AwsUser, AccessKey); err != nil {
			return err
		}
	}

	return nil
}

// UpgradeInRunTime upgrades the cluster in runtime inside a test
func UpgradeInRunTime(installType, value string) error {
	cmd := fmt.Sprintf("curl -sfL https://get.rke2.io | %s=%s "+
		"INSTALL_RKE2_EXEC=master sh -",
		installType, value)

	nodeExternalIps := FetchNodeExternalIP()
	for _, ip := range nodeExternalIps {
		if _, err := RunCommandOnNode(cmd, ip, AwsUser, AccessKey); err != nil {
			return err
		}
	}

	return nil
}

// FetchIngressIP returns the ingress IP of the given namespace
func FetchIngressIP(namespace string) ([]string, error) {
	cmd := "kubectl get ingress -n " + namespace +
		" -o jsonpath='{.items[0].status.loadBalancer.ingress[*].ip}' --kubeconfig=" + KubeConfigFile
	res, err := RunCommandHost(cmd)
	if err != nil {
		return nil, err
	}
	ingressIP := strings.Trim(res, " ")
	if ingressIP != "" {
		ingressIPs := strings.Split(ingressIP, " ")
		return ingressIPs, nil
	}

	return nil, nil
}

func parseNodes(print bool, cmd string) ([]Node, error) {
	nodes := make([]Node, 0, 10)
	res, err := RunCommandHost(cmd)
	if err != nil {
		return nil, err
	}
	rawNodes := strings.TrimSpace(res)
	split := strings.Split(rawNodes, "\n")
	for _, rec := range split {
		if strings.TrimSpace(rec) != "" {
			fields := strings.Fields(rec)
			n := Node{
				Name:       fields[0],
				Status:     fields[1],
				Roles:      fields[2],
				Version:    fields[4],
				InternalIP: fields[5],
				ExternalIP: fields[6],
			}
			nodes = append(nodes, n)
		}
	}
	if print {
		fmt.Println(rawNodes)
	}

	return nodes, nil
}

func parsePods(print bool, cmd string) ([]Pod, error) {
	pods := make([]Pod, 0, 10)
	res, err := RunCommandHost(cmd)
	if err != nil {
		return nil, rke2error.NewRke2Error("runCommandHost",
			res, "error: %v", err)
	}
	rawPods := strings.TrimSpace(res)

	split := strings.Split(rawPods, "\n")
	for _, rec := range split {
		fields := strings.Fields(string(rec))
		p := Pod{
			NameSpace: fields[0],
			Name:      fields[1],
			Ready:     fields[2],
			Status:    fields[3],
			Restarts:  fields[4],
			NodeIP:    fields[6],
			Node:      fields[7],
		}
		pods = append(pods, p)
	}
	if print {
		fmt.Println(rawPods)
	}

	return pods, nil
}
