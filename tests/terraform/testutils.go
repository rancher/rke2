package terraform

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
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

var config *ssh.ClientConfig
var SSHKEY string
var SSHUSER string
var err error

func basepath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(b), "../..")
}

func printFileContents(f string) {
	content, err := os.ReadFile(f)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(content))
}

func checkError(e error) {
	if e != nil {
		log.Fatal(err)
		panic(e)
	}
}

func publicKey(path string) ssh.AuthMethod {
	key, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		panic(err)
	}
	return ssh.PublicKeys(signer)
}

func configureSSH(host string, SSHUser string, SSHKey string) *ssh.Client {
	config = &ssh.ClientConfig{
		User: SSHUser,
		Auth: []ssh.AuthMethod{
			publicKey(SSHKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	conn, err := ssh.Dial("tcp", host, config)
	checkError(err)
	return conn
}

func runsshCommand(cmd string, conn *ssh.Client) (string, error) {
	session, err := conn.NewSession()
	if err != nil {
		panic(err)
	}
	defer session.Close()
	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf
	if err := session.Run(cmd); err != nil {
		log.Println(session.Stdout)
		log.Fatal("Error on command execution", err.Error())
	}
	return fmt.Sprintf("%s", stdoutBuf.String()), err
}

// RunCmdOnNode executes a command from within the given node
func runCmdOnNode(cmd string, ServerIP string, SSHUser string, SSHKey string) (string, error) {
	Server := ServerIP + ":22"
	conn := configureSSH(Server, SSHUser, SSHKey)
	res, err := runsshCommand(cmd, conn)
	res = strings.TrimSpace(res)
	return res, err
}

// runCommand executes a command on the host
func runCommand(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	out, err := c.CombinedOutput()
	return string(out), err
}

// Used to count the pods using prefix passed in the list of pods
func countOfStringInSlice(str string, pods []Pod) int {
	count := 0
	for _, pod := range pods {
		if strings.Contains(pod.Name, str) {
			count++
		}
	}
	return count
}

func deployWorkload(workload, kubeconfig string) (string, error) {
	resourceDir := basepath() + "/tests/terraform/resource_files"
	files, err := ioutil.ReadDir(resourceDir)
	if err != nil {
		err = fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)
		return "", err
	}
	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			cmd := "kubectl apply -f " + filename + " --kubeconfig=" + kubeconfig
			return runCommand(cmd)
		}
	}
	return "", nil
}

func removeWorkload(workload, kubeconfig string) (string, error) {
	resourceDir := basepath() + "/tests/terraform/resource_files"
	files, err := ioutil.ReadDir(resourceDir)
	if err != nil {
		err = fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)
		return "", err
	}
	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			cmd := "kubectl delete -f " + filename + " --kubeconfig=" + kubeconfig
			return runCommand(cmd)
		}
	}
	return "", nil
}

func fetchClusterIP(kubeconfig string, namespace string, servicename string) (string, string, error) {
	ipCmd := "kubectl get svc " + servicename + " -n " + namespace + " -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + kubeconfig
	ip, err := runCommand(ipCmd)
	if err != nil {
		return "", "", err
	}
	portCmd := "kubectl get svc " + servicename + " -n " + namespace + " -o jsonpath='{.spec.ports[0].port}' --kubeconfig=" + kubeconfig
	port, err := runCommand(portCmd)
	if err != nil {
		return "", "", err
	}
	return ip, port, err
}

func fetchNodeExternalIP(kubeconfig string) []string {
	cmd := "kubectl get node --output=jsonpath='{range .items[*]} { .status.addresses[?(@.type==\"ExternalIP\")].address}' --kubeconfig=" + kubeconfig
	time.Sleep(10 * time.Second)
	res, _ := runCommand(cmd)
	nodeExternalIP := strings.Trim(res, " ")
	nodeExternalIPs := strings.Split(nodeExternalIP, " ")
	return nodeExternalIPs
}

func fetchIngressIP(namespace string, kubeconfig string) ([]string, error) {
	cmd := "kubectl get ingress -n " + namespace + " -o jsonpath='{.items[0].status.loadBalancer.ingress[*].ip}' --kubeconfig=" + kubeconfig
	res, err := runCommand(cmd)
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

func parseNodes(kubeConfig string, print bool, cmd string) ([]Node, error) {
	nodes := make([]Node, 0, 10)
	res, err := runCommand(cmd)
	if err != nil {
		return nil, err
	}
	rawNodes := strings.TrimSpace(res)
	split := strings.Split(rawNodes, "\n")
	for _, rec := range split {
		if strings.TrimSpace(rec) != "" {
			fields := strings.Fields(rec)
			node := Node{
				Name:       fields[0],
				Status:     fields[1],
				Roles:      fields[2],
				Version:    fields[4],
				InternalIP: fields[5],
				ExternalIP: fields[6],
			}
			nodes = append(nodes, node)
		}
	}
	if print {
		fmt.Println(rawNodes)
	}
	return nodes, nil
}

func nodes(kubeConfig string, print bool) ([]Node, error) {
	cmd := "kubectl get nodes --no-headers -o wide --kubeconfig=" + kubeConfig
	return parseNodes(kubeConfig, print, cmd)
}

func workerNodes(kubeConfig string, print bool) ([]Node, error) {
	cmd := "kubectl get node -o jsonpath='{range .items[*]}{@.metadata.name} {@.status.conditions[-1].type} <not retrieved> <not retrieved> {@.status.nodeInfo.kubeletVersion} {@.status.addresses[?(@.type==\"InternalIP\")].address} {@.status.addresses[?(@.type==\"ExternalIP\")].address} {@.spec.taints[*].effect}{\"\\n\"}{end}' --kubeconfig=" + kubeConfig + " | grep -v NoSchedule | grep -v NoExecute"
	return parseNodes(kubeConfig, print, cmd)
}

func parsePods(kubeconfig string, print bool, cmd string) ([]Pod, error) {
	pods := make([]Pod, 0, 10)
	res, _ := runCommand(cmd)
	rawPods := strings.TrimSpace(res)

	split := strings.Split(rawPods, "\n")
	for _, rec := range split {
		fields := strings.Fields(string(rec))
		pod := Pod{
			NameSpace: fields[0],
			Name:      fields[1],
			Ready:     fields[2],
			Status:    fields[3],
			Restarts:  fields[4],
			NodeIP:    fields[6],
			Node:      fields[7],
		}
		pods = append(pods, pod)
	}
	if print {
		fmt.Println(rawPods)
	}
	return pods, nil
}

func pods(kubeconfig string, print bool) ([]Pod, error) {
	cmd := "kubectl get pods -o wide --no-headers -A --kubeconfig=" + kubeconfig
	return parsePods(kubeconfig, print, cmd)
}
