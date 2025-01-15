package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo/v2"
	"golang.org/x/sync/errgroup"
)

type Node struct {
	Name       string
	Status     string
	Roles      string
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

type NodeError struct {
	Node string
	Cmd  string
	Err  error
}

type objIP struct {
	Name string
	Ipv4 string
	Ipv6 string
}

func (ne *NodeError) Error() string {
	return fmt.Sprintf("failed creating cluster: %s: %v", ne.Cmd, ne.Err)
}

func (ne *NodeError) Unwrap() error {
	return ne.Err
}

func newNodeError(cmd, node string, err error) *NodeError {
	return &NodeError{
		Cmd:  cmd,
		Node: node,
		Err:  err,
	}
}

func CountOfStringInSlice(str string, pods []Pod) int {
	count := 0
	for _, pod := range pods {
		if strings.Contains(pod.Name, str) {
			count++
		}
	}
	return count
}

// genNodeEnvs generates the node and testing environment variables for vagrant up
func genNodeEnvs(nodeOS string, serverCount, agentCount, windowsAgentCount int) ([]string, []string, []string, string) {
	serverNodeNames := make([]string, serverCount)
	for i := 0; i < serverCount; i++ {
		serverNodeNames[i] = "server-" + strconv.Itoa(i)
	}
	var agentPrefix string
	if windowsAgentCount > 0 {
		agentPrefix = "linux-"
	}
	agentNodeNames := make([]string, agentCount)
	for i := 0; i < agentCount; i++ {
		agentNodeNames[i] = agentPrefix + "agent-" + strconv.Itoa(i)
	}

	windowsAgentNames := make([]string, windowsAgentCount)
	for i := 0; i < windowsAgentCount; i++ {
		windowsAgentNames[i] = "windows-agent-" + strconv.Itoa(i)
	}

	nodeRoles := strings.Join(serverNodeNames, " ") + " " + strings.Join(agentNodeNames, " ") + " " + strings.Join(windowsAgentNames, " ")
	nodeRoles = strings.TrimSpace(nodeRoles)

	nodeBoxes := strings.Repeat(nodeOS+" ", serverCount+agentCount)
	nodeBoxes = nodeBoxes + strings.Repeat("jborean93/WindowsServer2022"+" ", windowsAgentCount)
	nodeBoxes = strings.TrimSpace(nodeBoxes)

	nodeEnvs := fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s"`, nodeRoles, nodeBoxes)

	return serverNodeNames, agentNodeNames, windowsAgentNames, nodeEnvs
}

func CreateCluster(nodeOS string, serverCount int, agentCount int) ([]string, []string, error) {

	serverNodeNames, agentNodeNames, _, nodeEnvs := genNodeEnvs(nodeOS, serverCount, agentCount, 0)

	var testOptions string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}

	// Bring up the first server node
	cmd := fmt.Sprintf(`%s %s vagrant up --no-tty %s &> vagrant.log`, nodeEnvs, testOptions, serverNodeNames[0])

	fmt.Println(cmd)
	if _, err := RunCommand(cmd); err != nil {
		return nil, nil, newNodeError(cmd, serverNodeNames[0], err)
	}

	// Bring up the rest of the nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	for _, node := range append(serverNodeNames[1:], agentNodeNames...) {
		cmd := fmt.Sprintf(`%s %s vagrant up --no-tty %s &>> vagrant.log`, nodeEnvs, testOptions, node)
		fmt.Println(cmd)
		errg.Go(func() error {
			if _, err := RunCommand(cmd); err != nil {
				return newNodeError(cmd, node, err)
			}
			return nil
		})
		// We must wait a bit between provisioning nodes to avoid too many learners attempting to join the cluster
		time.Sleep(40 * time.Second)
	}
	if err := errg.Wait(); err != nil {
		return nil, nil, err
	}

	return serverNodeNames, agentNodeNames, nil
}

func CreateMixedCluster(nodeOS string, serverCount, linuxAgentCount, windowsAgentCount int) ([]string, []string, []string, error) {
	serverNodeNames, linuxAgentNames, windowsAgentNames, nodeEnvs := genNodeEnvs(nodeOS, serverCount, linuxAgentCount, windowsAgentCount)

	var testOptions string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}

	cmd := fmt.Sprintf("%s %s vagrant up --no-tty &> vagrant.log", nodeEnvs, testOptions)
	fmt.Println(cmd)
	if _, err := RunCommand(cmd); err != nil {
		fmt.Println("Error Creating Cluster", err)
		return nil, nil, nil, err
	}
	return serverNodeNames, linuxAgentNames, windowsAgentNames, nil
}

func scpRKE2Artifacts(nodeNames []string) error {
	binary := []string{
		"dist/artifacts/rke2.linux-amd64.tar.gz",
		"dist/artifacts/sha256sum-amd64.txt",
	}
	images := []string{
		"build/images/rke2-images.linux-amd64.tar.zst",
	}

	// vagrant scp doesn't allow coping multiple files at once
	// nor does it allow copying as sudo, so we have to copy each file individually
	// to /tmp/ and then move them to the correct location
	for _, node := range nodeNames {
		for _, artifact := range append(binary, images...) {
			cmd := fmt.Sprintf(`vagrant scp ../../../%s %s:/tmp/`, artifact, node)
			if _, err := RunCommand(cmd); err != nil {
				return err
			}
		}
		if _, err := RunCmdOnNode("mkdir -p /var/lib/rancher/rke2/agent/images", node); err != nil {
			return err
		}
		for _, image := range images {
			cmd := fmt.Sprintf("mv /tmp/%s /var/lib/rancher/rke2/agent/images/", filepath.Base(image))
			if _, err := RunCmdOnNode(cmd, node); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateLocalCluster creates a cluster using the locally built RKE2 bundled binary and images.
// Run at a minimum "make package-bundle" and "make package-image-runtime" first
// The vagrant-scp plugin must be installed for this function to work.
func CreateLocalCluster(nodeOS string, serverCount, agentCount int) ([]string, []string, error) {

	serverNodeNames, agentNodeNames, _, nodeEnvs := genNodeEnvs(nodeOS, serverCount, agentCount, 0)

	var testOptions string

	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}
	testOptions += " E2E_RELEASE_VERSION=skip"

	// Standup all VMs. In GitHub Actions, this also imports the VM image into libvirt, which takes time to complete.
	cmd := fmt.Sprintf(`%s %s E2E_STANDUP_PARALLEL=true vagrant up --no-tty --no-provision &> vagrant.log`, nodeEnvs, testOptions)
	fmt.Println(cmd)
	if _, err := RunCommand(cmd); err != nil {
		return nil, nil, newNodeError(cmd, serverNodeNames[0], err)
	}

	if err := scpRKE2Artifacts(append(serverNodeNames, agentNodeNames...)); err != nil {
		return nil, nil, err
	}
	// Install RKE2 on all nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	for _, node := range append(serverNodeNames, agentNodeNames...) {
		cmd = fmt.Sprintf(`%s %s vagrant provision %s &>> vagrant.log`, nodeEnvs, testOptions, node)
		errg.Go(func() error {
			if _, err := RunCommand(cmd); err != nil {
				return newNodeError(cmd, node, err)
			}
			return nil
		})
		// RKE2 needs some time between joining nodes to avoid learner issues
		time.Sleep(20 * time.Second)
	}
	if err := errg.Wait(); err != nil {
		return nil, nil, err
	}
	return serverNodeNames, agentNodeNames, nil
}

func scpWindowsRKE2Artifacts(nodeNames []string) error {
	binary := []string{
		"dist/artifacts/rke2.windows-amd64.tar.gz",
		"dist/artifacts/sha256sum-windows-amd64.txt",
		"bin/rke2.exe",
	}
	images := []string{
		"build/images/rke2-images.windows-amd64.tar.zst",
	}

	// vagrant scp doesn't allow coping multiple files at once
	// nor does it allow copying as sudo, so we have to copy each file individually
	// to /temp and then move them to the correct location
	for _, node := range nodeNames {
		for _, artifact := range append(binary, images...) {
			cmd := fmt.Sprintf(`vagrant scp ../../../%s %s:/temp/`, artifact, node)
			if _, err := RunCommand(cmd); err != nil {
				return err
			}
		}
		if _, err := RunCmdOnWindowsNode(`mv C:\temp\sha256sum-windows-amd64.txt C:\temp\sha256sum-amd64.txt`, node); err != nil {
			return err
		}
		if _, err := RunCmdOnWindowsNode(`mkdir C:\var\lib\rancher\rke2\agent\images`, node); err != nil {
			return err
		}

		for _, image := range images {
			cmd := fmt.Sprintf("mv C:\\temp\\%s C:\\var\\lib\\rancher\\rke2\\agent\\images\\ ", filepath.Base(image))
			if _, err := RunCmdOnWindowsNode(cmd, node); err != nil {
				return err
			}
		}
		if _, err := RunCmdOnWindowsNode(`mkdir C:\usr\local\bin`, node); err != nil {
			return err
		}
		if _, err := RunCmdOnWindowsNode(`mv C:\temp\rke2.exe C:\usr\local\bin\rke2.exe`, node); err != nil {
			return err
		}
	}
	return nil
}

func CreateLocalMixedCluster(nodeOS string, serverCount, linuxAgentCount, windowsAgentCount int) ([]string, []string, []string, error) {
	serverNodeNames, linuxAgentNames, windowsAgentNames, nodeEnvs := genNodeEnvs(nodeOS, serverCount, linuxAgentCount, windowsAgentCount)

	var testOptions string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}
	testOptions += " E2E_RELEASE_VERSION=skip"

	// Standup all nodes, relying on vagrant-libvirt native parallel provisioning
	cmd := fmt.Sprintf(`%s %s E2E_STANDUP_PARALLEL=true vagrant up --no-tty --no-provision &> vagrant.log`, nodeEnvs, testOptions)
	fmt.Println(cmd)
	if _, err := RunCommand(cmd); err != nil {
		return nil, nil, nil, err
	}

	if err := scpRKE2Artifacts(append(serverNodeNames, linuxAgentNames...)); err != nil {
		return nil, nil, nil, err
	}
	if err := scpWindowsRKE2Artifacts(windowsAgentNames); err != nil {
		return nil, nil, nil, err
	}
	// Install RKE2 on all nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	allNodes := append(serverNodeNames, linuxAgentNames...)
	allNodes = append(allNodes, windowsAgentNames...)
	for _, node := range allNodes {
		cmd = fmt.Sprintf(`%s %s vagrant provision %s &>> vagrant.log`, nodeEnvs, testOptions, node)
		errg.Go(func() error {
			if _, err := RunCommand(cmd); err != nil {
				return newNodeError(cmd, node, err)
			}
			return nil
		})
		// RKE2 needs some time between joining nodes to avoid learner issues
		time.Sleep(20 * time.Second)
	}
	if err := errg.Wait(); err != nil {
		return nil, nil, nil, err
	}
	return serverNodeNames, linuxAgentNames, windowsAgentNames, nil
}

func DeployWorkload(workload string, kubeconfig string) (string, error) {
	resourceDir := "../resource_files"
	files, err := os.ReadDir(resourceDir)
	if err != nil {
		fmt.Println("Unable to read resource manifest file for ", workload)
	}
	fmt.Println("\nDeploying", workload)
	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			cmd := "kubectl apply -f " + filename + " --kubeconfig=" + kubeconfig
			return RunCommand(cmd)
		}
	}
	return "", nil
}

// RestartCluster restarts the rke2 service on each server-agent given
func RestartCluster(nodeNames []string) error {
	for _, nodeName := range nodeNames {
		const cmd = "systemctl restart rke2-*"
		if _, err := RunCmdOnNode(cmd, nodeName); err != nil {
			return err
		}
	}
	return nil
}

func DestroyCluster() error {
	if _, err := RunCommand("vagrant destroy -f"); err != nil {
		return err
	}
	return os.Remove("vagrant.log")
}

func FetchClusterIP(kubeconfig string, servicename string, dualStack bool) (string, error) {
	if dualStack {
		cmd := "kubectl get svc " + servicename + " -o jsonpath='{.spec.clusterIPs}' --kubeconfig=" + kubeconfig
		res, err := RunCommand(cmd)
		if err != nil {
			return res, err
		}
		res = strings.ReplaceAll(res, "\"", "")
		return strings.Trim(res, "[]"), nil
	}
	cmd := "kubectl get svc " + servicename + " -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + kubeconfig
	return RunCommand(cmd)
}

func FetchIngressIP(kubeconfig string) ([]string, error) {
	cmd := "kubectl get ing  ingress  -o jsonpath='{.status.loadBalancer.ingress[*].ip}' --kubeconfig=" + kubeconfig
	res, err := RunCommand(cmd)
	if err != nil {
		return nil, err
	}
	ingressIP := strings.Trim(res, " ")
	ingressIPs := strings.Split(ingressIP, " ")
	return ingressIPs, nil
}

func FetchNodeExternalIP(nodename string) (string, error) {
	cmd := "vagrant ssh " + nodename + " -c  \"ip -f inet addr show eth1| awk '/inet / {print $2}'|cut -d/ -f1\""
	ipaddr, err := RunCommand(cmd)
	if err != nil {
		return "", err
	}
	ips := strings.Trim(ipaddr, "")
	ip := strings.Split(ips, "inet")
	nodeip := strings.TrimSpace(ip[1])
	return nodeip, nil
}

// GenReport returns the relevant lines from test results in json format
func GenReport(specReport ginkgo.SpecReport) {
	state := struct {
		State string        `json:"state"`
		Name  string        `json:"name"`
		Type  string        `json:"type"`
		Time  time.Duration `json:"time"`
	}{
		State: specReport.State.String(),
		Name:  specReport.LeafNodeText,
		Type:  "rke2 test",
		Time:  specReport.RunTime,
	}
	status, _ := json.Marshal(state)
	fmt.Printf("%s", status)
}

// GetVagrantLog returns the logs of on vagrant commands that initialize the nodes and provision RKE2 on each node.
// It also attempts to fetch the systemctl and kubelet/containerd logs of RKE2 on nodes where the rke2.service failed.
func GetVagrantLog(cErr error) string {
	var nodeErr *NodeError
	nodeJournal := ""
	if errors.As(cErr, &nodeErr) {
		if strings.Contains(nodeErr.Node, "windows-agent") {
			nodeJournal, _ = RunCmdOnWindowsNode("Get-EventLog -LogName Application -Source 'rke2'", nodeErr.Node)
		} else {
			nodeJournal, _ = RunCmdOnNode("sudo journalctl -u rke2* --no-pager", nodeErr.Node)
		}
		nodeJournal = "\nNode Journal Logs:\n" + nodeJournal

		paths := []string{"/var/lib/rancher/rke2/agent/logs/kubelet.log", "/var/lib/rancher/rke2/agent/containerd/containerd.log"}
		for _, path := range paths {
			out, _ := RunCmdOnNode("sudo cat "+path, nodeErr.Node)
			nodeJournal += "\n" + path + ":\n" + out + "\n"
		}
	}

	log, err := os.Open("vagrant.log")
	if err != nil {
		return err.Error()
	}
	bytes, err := io.ReadAll(log)
	if err != nil {
		return err.Error()
	}
	return string(bytes) + nodeJournal
}

func GenKubeConfigFile(serverName string) (string, error) {
	kubeConfig, err := RunCmdOnNode("cat /etc/rancher/rke2/rke2.yaml", serverName)
	if err != nil {
		return "", err
	}

	nodeIP, err := FetchNodeExternalIP(serverName)
	if err != nil {
		return "", err
	}
	kubeConfig = strings.Replace(kubeConfig, "127.0.0.1", nodeIP, 1)
	kubeConfigFile := fmt.Sprintf("kubeconfig-%s", serverName)
	if err := os.WriteFile(kubeConfigFile, []byte(kubeConfig), 0644); err != nil {
		return "", err
	}
	return kubeConfigFile, nil
}

func ParseNodes(kubeConfig string, print bool) ([]Node, error) {
	nodes := make([]Node, 0, 10)
	nodeList := ""

	cmd := "kubectl get nodes --no-headers -o wide -A --kubeconfig=" + kubeConfig
	res, err := RunCommand(cmd)

	if err != nil {
		return nil, fmt.Errorf("failed cmd: %s, %w", cmd, err)
	}
	nodeList = strings.TrimSpace(res)
	split := strings.Split(nodeList, "\n")
	for _, rec := range split {
		if strings.TrimSpace(rec) != "" {
			fields := strings.Fields(rec)
			node := Node{
				Name:       fields[0],
				Status:     fields[1],
				Roles:      fields[2],
				InternalIP: fields[5],
				ExternalIP: fields[6],
			}
			nodes = append(nodes, node)
		}
	}
	if print {
		fmt.Println(nodeList)
	}
	return nodes, nil
}

func ParsePods(kubeconfig string, print bool) ([]Pod, error) {
	pods := make([]Pod, 0, 10)
	podList := ""

	cmd := "kubectl get pods -o wide --no-headers -A --kubeconfig=" + kubeconfig
	res, _ := RunCommand(cmd)
	res = strings.TrimSpace(res)
	podList = res

	split := strings.Split(res, "\n")
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
		fmt.Println(podList)
	}
	return pods, nil
}

// RunCmdOnNode executes a command from within the given node as sudo
func RunCmdOnNode(cmd string, nodename string) (string, error) {
	runcmd := "vagrant ssh -c \"sudo " + cmd + "\" " + nodename
	out, err := RunCommand(runcmd)
	// On GHA CI we see warnings about "[fog][WARNING] Unrecognized arguments: libvirt_ip_command"
	// these are added to the command output and need to be removed
	out = strings.ReplaceAll(out, "[fog][WARNING] Unrecognized arguments: libvirt_ip_command\n", "")
	if err != nil {
		return out, fmt.Errorf("failed to run command: %s on node %s: %s, %v", cmd, nodename, out, err)
	}
	return out, nil
}

// RunCmdOnWindowsNode executes a command from within the given windows node
func RunCmdOnWindowsNode(cmd string, nodename string) (string, error) {
	runcmd := "vagrant ssh -c 'powershell.exe -Command \"" + cmd + "\"' " + nodename
	out, err := RunCommand(runcmd)
	if err != nil {
		return out, fmt.Errorf("failed to run windows command: %s : out : %v", runcmd, err)
	}
	return out, nil
}

// RunCommand execute a command on the host
func RunCommand(cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*15)
	defer cancel()
	c := exec.CommandContext(ctx, "bash", "-c", cmd)
	out, err := c.CombinedOutput()
	return string(out), err
}

// StartCluster starts the rke2 service on each node given
func StartCluster(nodeNames []string) error {
	for _, nodeName := range nodeNames {
		cmd := "systemctl start rke2"
		if strings.Contains(nodeName, "server") {
			cmd += "-server"
		}
		if strings.Contains(nodeName, "agent") {
			cmd += "-agent"
		}
		if _, err := RunCmdOnNode(cmd, nodeName); err != nil {
			return err
		}
	}
	return nil
}

// StopCluster starts the rke2 service on each node given
func StopCluster(nodeNames []string) error {
	for _, nodeName := range nodeNames {
		cmd := "systemctl stop rke2*"
		if _, err := RunCmdOnNode(cmd, nodeName); err != nil {
			return err
		}
	}
	return nil
}

func UpgradeCluster(serverNodenames []string, agentNodenames []string) error {
	for _, nodeName := range serverNodenames {
		cmd := "E2E_RELEASE_CHANNEL=commit vagrant provision " + nodeName
		fmt.Println(cmd)
		if out, err := RunCommand(cmd); err != nil {
			fmt.Println("Error Upgrading Cluster", out)
			return err
		}
	}
	for _, nodeName := range agentNodenames {
		cmd := "E2E_RELEASE_CHANNEL=commit vagrant provision " + nodeName
		if _, err := RunCommand(cmd); err != nil {
			fmt.Println("Error Upgrading Cluster", err)
			return err
		}
	}
	return nil
}

// PodIPsUsingLabel returns the IPs of the pods with a label
func PodIPsUsingLabel(kubeConfigFile string, label string) ([]objIP, error) {
	cmd := `kubectl get pods -l ` + label + ` -o=jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.podIPs[*].ip}{"\n"}{end}' --kubeconfig=` + kubeConfigFile
	return getObjIPs(cmd)
}

// GetPodIPs returns the IPs of all the pods
func GetPodIPs(kubeConfigFile string) ([]objIP, error) {
	cmd := `kubectl get pods -A -o=jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.podIPs[*].ip}{"\n"}{end}' --kubeconfig=` + kubeConfigFile
	return getObjIPs(cmd)
}

// GetNodeIPs returns the IPs of the nodes
func GetNodeIPs(kubeConfigFile string) ([]objIP, error) {
	cmd := `kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.addresses[?(@.type == "ExternalIP")].address}{"\n"}{end}' --kubeconfig=` + kubeConfigFile
	return getObjIPs(cmd)
}

// getObjIPs processes the IPs of the requested objects
func getObjIPs(cmd string) ([]objIP, error) {
	var objIPs []objIP
	res, err := RunCommand(cmd)
	if err != nil {
		return nil, err
	}
	objs := strings.Split(res, "\n")
	objs = objs[:len(objs)-1]

	for _, obj := range objs {
		fields := strings.Fields(obj)
		if len(fields) > 2 {
			objIPs = append(objIPs, objIP{Name: fields[0], Ipv4: fields[1], Ipv6: fields[2]})
		} else if len(fields) > 1 {
			objIPs = append(objIPs, objIP{Name: fields[0], Ipv4: fields[1]})
		} else {
			objIPs = append(objIPs, objIP{Name: fields[0]})
		}
	}
	return objIPs, nil
}
