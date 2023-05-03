package terraform

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type KubectlCommand string

var config *ssh.ClientConfig

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

func publicKey(path string) (ssh.AuthMethod, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

func configureSSH(host string, sshUser string, sshKey string) (*ssh.Client, error) {
	authMethod, err := publicKey(sshKey)
	if err != nil {
		return nil, err
	}
	config = &ssh.ClientConfig{
		User: sshUser,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	conn, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func runsshCommand(cmd string, conn *ssh.Client) (string, error) {
	session, err := conn.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf
	if err := session.Run(cmd); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s", stdoutBuf.String()), err
}

func parseNodes(kubeConfig string, print bool, cmd string) ([]Node, error) {
	nodes := make([]Node, 0, 10)
	res, err := RunCommand(cmd)
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

func parsePods(kubeconfig string, print bool, cmd string) ([]Pod, error) {
	pods := make([]Pod, 0, 10)
	res, _ := RunCommand(cmd)
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

func Basepath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(b), "../..")
}

func PrintFileContents(f ...string) error {
	for _, file := range f {
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		fmt.Println(string(content) + "\n")
	}

	return nil
}

// RunCommandOnNode executes a command from within the given node
func RunCommandOnNode(cmd string, ServerIP string, sshUser string, sshKey string) (string, error) {
	Server := ServerIP + ":22"
	conn, err := configureSSH(Server, sshUser, sshKey)
	if err != nil {
		return "", err
	}
	res, err := runsshCommand(cmd, conn)
	res = strings.TrimSpace(res)

	return res, err
}

// RunCommand executes a command on the host
func RunCommand(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	out, err := c.CombinedOutput()

	return string(out), err
}

// CountOfStringInSlice Used to count the pods using prefix passed in the list of pods
func CountOfStringInSlice(str string, pods []Pod) int {
	var count int
	for _, p := range pods {
		if strings.Contains(p.Name, str) {
			count++
		}
	}

	return count
}

func DeployWorkload(workload, kubeconfig string) (string, error) {
	resourceDir := Basepath() + "/tests/terraform/resource_files"
	files, err := os.ReadDir(resourceDir)
	if err != nil {
		return "", fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)
	}
	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			cmd := "kubectl apply -f " + filename + " --kubeconfig=" + kubeconfig
			return RunCommand(cmd)
		}
	}

	return "", nil
}

func RemoveWorkload(workload, kubeconfig string) (string, error) {
	resourceDir := Basepath() + "/tests/terraform/resource_files"
	files, err := os.ReadDir(resourceDir)
	if err != nil {
		return "", fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)
	}
	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			cmd := "kubectl delete -f " + filename + " --kubeconfig=" + kubeconfig
			return RunCommand(cmd)
		}
	}

	return "", nil
}

func FetchClusterIP(kubeconfig string, namespace string, servicename string) (string, string, error) {
	ipCmd := "kubectl get svc " + servicename + " -n " + namespace +
		" -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + kubeconfig
	ip, err := RunCommand(ipCmd)
	if err != nil {
		return "", "", err
	}
	portCmd := "kubectl get svc " + servicename + " -n " + namespace +
		" -o jsonpath='{.spec.ports[0].port}' --kubeconfig=" + kubeconfig
	port, err := RunCommand(portCmd)
	if err != nil {
		return "", "", err
	}

	return ip, port, err
}

func FetchNodeExternalIP(kubeconfig string) []string {
	cmd := "kubectl get node --output=jsonpath='{range .items[*]} " +
		"{ .status.addresses[?(@.type==\"ExternalIP\")].address}' --kubeconfig=" + kubeconfig
	time.Sleep(10 * time.Second)
	res, _ := RunCommand(cmd)
	nodeExternalIP := strings.Trim(res, " ")
	nodeExternalIPs := strings.Split(nodeExternalIP, " ")

	return nodeExternalIPs
}

func FetchIngressIP(namespace string, kubeconfig string) ([]string, error) {
	cmd := "kubectl get ingress -n " + namespace +
		" -o jsonpath='{.items[0].status.loadBalancer.ingress[*].ip}' --kubeconfig=" + kubeconfig
	res, err := RunCommand(cmd)
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

func Nodes(kubeConfig string, print bool) ([]Node, error) {
	cmd := "kubectl get nodes --no-headers -o wide --kubeconfig=" + kubeConfig
	return parseNodes(kubeConfig, print, cmd)
}

func WorkerNodes(kubeConfig string, print bool) ([]Node, error) {
	cmd := "kubectl get node -o jsonpath='{range .items[*]}{@.metadata.name} " +
		"{@.status.conditions[-1].type} <not retrieved> <not retrieved> {@.status.nodeInfo.kubeletVersion} " +
		"{@.status.addresses[?(@.type==\"InternalIP\")].address} " +
		"{@.status.addresses[?(@.type==\"ExternalIP\")].address} {@.spec.taints[*].effect}{\"\\n\"}{end}' " +
		"--kubeconfig=" + kubeConfig + " | grep -v NoSchedule | grep -v NoExecute"

	return parseNodes(kubeConfig, print, cmd)
}

func Pods(kubeconfig string, print bool) ([]Pod, error) {
	cmd := "kubectl get pods -o wide --no-headers -A --kubeconfig=" + kubeconfig
	return parsePods(kubeconfig, print, cmd)
}

func IsAppRunning(namespace, appName string, kubeconfig string) (string, error) {
	cmd := "kubectl get pods -n" + namespace + " -o=name -l k8s-app=" + appName + " --field-selector=status.phase=Running --kubeconfig=" + kubeconfig
	return RunCommand(cmd)
}
