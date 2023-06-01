package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/onsi/gomega"
)

var (
	KubeConfigFile string
	AwsUser        string
	AccessKey      string
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

const (
	GrepNoExec    = " | grep -v NoSchedule | grep -v NoExecute"
	RunningAssert = "Running"
)

// ManageWorkload creates or deletes a workload based on the action: create or delete.
func ManageWorkload(action, workload string) (string, error) {
	if action != "create" && action != "delete" {
		return "", fmt.Errorf("invalid action: %s. Must be 'create' or 'delete'", action)
	}
	var res string
	var err error

	resourceDir := BasePath() + "/acceptance/workloads/"

	files, err := os.ReadDir(resourceDir)
	if err != nil {
		err = fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)
		return "", err
	}

	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			if action == "create" {
				res, err = createWorkload(workload, filename)
				if err != nil {
					return "", fmt.Errorf("failed to create workload %s: %s", workload, err)
				}
			} else {
				res, err = deleteWorkload(workload, filename)
				if err != nil {
					return "", fmt.Errorf("failed to delete workload %s: %s", workload, err)
				}
			}
			return res, err
		}
	}

	return "", fmt.Errorf("workload %s not found", workload)
}

// createWorkload creates a workload.
func createWorkload(workload, filename string) (string, error) {
	fmt.Println("\nDeploying", workload)
	return RunCommandHost("kubectl apply -f " + filename + " --kubeconfig=" + KubeConfigFile)

}

// deleteWorkload deletes a workload and asserts that the workload is deleted.
func deleteWorkload(workload, filename string) (string, error) {
	fmt.Println("\nRemoving", workload)
	cmd := "kubectl delete -f " + filename + " --kubeconfig=" + KubeConfigFile

	gomega.Eventually(func(g gomega.Gomega) {
		isDeleted, err := IsWorkloadDeleted(workload)
		g.Expect(err).To(gomega.BeNil())
		g.Expect(isDeleted).To(gomega.BeTrue(),
			"Workload should be deleted")
	}, "60s", "5s").Should(gomega.Succeed())

	return RunCommandHost(cmd)
}

// IsWorkloadDeleted returns true if the workload is deleted.
func IsWorkloadDeleted(workload string) (bool, error) {
	res, err := RunCommandHost("kubectl get all -A --kubeconfig=" + KubeConfigFile)
	if err != nil {
		return false, err
	}

	return !strings.Contains(res, workload), nil
}

// KubectlCommand return results from various commands, it receives an "action" , source and args.
// it already has KubeConfigFile
//
// destination = host or node
//
// action = get,describe...
//
// source = pods, node , exec, service ...
//
// args   = the rest of your command arguments.
func KubectlCommand(destination, action, source string, args ...string) (string, error) {
	var cmd string
	var res string
	var err error
	kubeconfigFlag := " --kubeconfig=" + KubeConfigFile

	if destination == "host" {
		cmd = addKubectlCommand(action, source, args) + kubeconfigFlag
		res, err = RunCommandHost(cmd)
		if err != nil {
			return "", err
		}
	} else if destination == "node" {
		cmd = addKubectlCommand(action, source, args) + kubeconfigFlag
		ips := FetchNodeExternalIP()
		for _, ip := range ips {
			res, err = RunCommandOnNode(cmd, ip)
			if err != nil {
				return "", err
			}
		}
	} else {
		return "", fmt.Errorf("invalid destination: %s", destination)
	}

	return res, nil
}

// addKubectlCommand using a specific action + source maps the args received  to create a kubectl command.
func addKubectlCommand(action, source string, args []string) string {
	shortCmd := map[string]string{
		"get":      "kubectl get",
		"describe": "kubectl describe",
		"exec":     "kubectl exec",
		"delete":   "kubectl delete",
		"apply":    "kubectl apply",
	}

	cmdPrefix, ok := shortCmd[action]
	if !ok {
		cmdPrefix = action
	}

	return cmdPrefix + " " + source + " " + strings.Join(args, " ")
}

// Nodes returns the list of nodes in the cluster and parses the output with parseNodes.
func Nodes(print bool) ([]Node, error) {
	return parseNodes("kubectl get nodes --no-headers -o wide --kubeconfig="+KubeConfigFile, print)
}

// WorkerNodes returns the list of worker nodes in the cluster.
func WorkerNodes(print bool) ([]Node, error) {
	return parseNodes("kubectl get node -o jsonpath='{range .items[*]}{@.metadata.name} "+
		"{@.status.conditions[-1].type} <not retrieved> <not retrieved> "+
		"{@.status.nodeInfo.kubeletVersion} "+
		"{@.status.addresses[?(@.type==\"InternalIP\")].address} "+
		"{@.status.addresses[?(@.type==\"ExternalIP\")].address} "+
		"{@.spec.taints[*].effect}{\"\\n\"}{end}' "+
		"--kubeconfig="+KubeConfigFile+GrepNoExec, print)
}

// Pods returns the list of pods in the cluster and parses the output with parsePods.
func Pods(print bool) ([]Pod, error) {
	return parsePods("kubectl get pods -o wide --no-headers -A --kubeconfig="+KubeConfigFile, print)
}

// FetchClusterIP returns the cluster IP and port of the service.
func FetchClusterIP(
	namespace string,
	serviceName string,
) (string, string, error) {
	ip, err := RunCommandHost("kubectl get svc " + serviceName + " -n " + namespace +
		" -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + KubeConfigFile)
	if err != nil {
		return "", "", err
	}

	port, err := RunCommandHost("kubectl get svc " + serviceName + " -n " + namespace +
		" -o jsonpath='{.spec.ports[0].port}' --kubeconfig=" + KubeConfigFile)
	if err != nil {
		return "", "", err
	}

	return ip, port, err
}

// FetchNodeExternalIP returns the external IP of the nodes.
func FetchNodeExternalIP() []string {
	res, _ := RunCommandHost("kubectl get nodes --output=jsonpath='{.items[*].status.addresses[?(@.type==\"ExternalIP\")].address}' " +
		"--kubeconfig=" + KubeConfigFile)
	nodeExternalIP := strings.Trim(res, " ")
	nodeExternalIPs := strings.Split(nodeExternalIP, " ")

	return nodeExternalIPs
}

// RestartCluster restarts the rke2 service on each node given by external IP.
func RestartCluster(ip string) (string, error) {
	return RunCommandOnNode("sudo systemctl restart rke2-*", ip)
}

// FetchIngressIP returns the ingress IP of the given namespace
func FetchIngressIP(namespace string) ([]string, error) {
	res, err := RunCommandHost(
		"kubectl get ingress -n " +
			namespace +
			"  -o jsonpath='{.items[0].status.loadBalancer.ingress[*].ip}' --kubeconfig=" +
			KubeConfigFile,
	)
	if err != nil {
		return nil, err
	}

	ingressIP := strings.Trim(res, " ")
	if ingressIP == "" {
		return nil, nil
	}
	ingressIPs := strings.Split(ingressIP, " ")

	return ingressIPs, nil
}

func parseNodes(cmd string, print bool) ([]Node, error) {
	nodes := make([]Node, 0, 10)

	res, err := RunCommandHost(cmd)
	if err != nil {
		return nil, err
	}

	nodelist := strings.TrimSpace(res)
	split := strings.Split(nodelist, "\n")
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
		fmt.Println(nodelist)
	}

	return nodes, nil
}

func parsePods(cmd string, print bool) ([]Pod, error) {
	pods := make([]Pod, 0, 10)

	res, err := RunCommandHost(cmd)
	if err != nil {
		return nil, err
	}

	podList := strings.TrimSpace(res)

	split := strings.Split(podList, "\n")
	for _, rec := range split {
		fields := strings.Fields(rec)
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
		fmt.Println(podList)
	}

	return pods, nil
}
