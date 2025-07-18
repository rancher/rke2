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

const (
	Linux   = iota
	Windows = iota
)

// defining the VagrantNode type allows methods like RunCmdOnNode to be defined on it.
// This makes test code more consistent, as similar functions can exists in Docker and E2E tests.
type VagrantNode struct {
	Name string
	Type int
}

func VagrantSlice(v []VagrantNode) []string {
	nodes := make([]string, 0, len(v))
	for _, node := range v {
		nodes = append(nodes, node.Name)
	}
	return nodes
}

type TestConfig struct {
	Hardened       bool
	KubeconfigFile string
	Servers        []VagrantNode
	Agents         []VagrantNode
	WindowsAgents  []VagrantNode
}

func (tc *TestConfig) AllNodes() []VagrantNode {
	return append(tc.Servers, tc.Agents...)
}

func (tc *TestConfig) Status() string {
	sN := strings.Join(VagrantSlice(tc.Servers), " ")
	aN := strings.Join(VagrantSlice(tc.Agents), " ")
	hardened := ""
	if tc.Hardened {
		hardened = "Hardened: true\n"
	}
	wN := ""
	if len(tc.WindowsAgents) > 0 {
		wN = fmt.Sprintf("Windows Agents: %s\n", strings.Join(VagrantSlice(tc.WindowsAgents), " "))
	}
	return fmt.Sprintf("%sKubeconfig: %s\nServers Nodes: %s\nAgents Nodes: %s\n%s)", hardened, tc.KubeconfigFile, sN, aN, wN)
}

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
	Cmd  string
	Node VagrantNode
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

func newNodeError(cmd string, node VagrantNode, err error) *NodeError {
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
func genNodeEnvs(nodeOS string, serverCount, agentCount, windowsAgentCount int) ([]VagrantNode, []VagrantNode, []VagrantNode, string) {
	serverNodes := make([]VagrantNode, serverCount)
	for i := 0; i < serverCount; i++ {
		serverNodes[i] = VagrantNode{"server-" + strconv.Itoa(i), Linux}
	}
	var agentPrefix string
	if windowsAgentCount > 0 {
		agentPrefix = "linux-"
	}
	agentNodes := make([]VagrantNode, agentCount)
	for i := 0; i < agentCount; i++ {
		agentNodes[i] = VagrantNode{agentPrefix + "agent-" + strconv.Itoa(i), Linux}
	}

	windowsAgentNodes := make([]VagrantNode, windowsAgentCount)
	for i := 0; i < windowsAgentCount; i++ {
		windowsAgentNodes[i] = VagrantNode{"windows-agent-" + strconv.Itoa(i), Windows}
	}

	nodeRoles := strings.Join(VagrantSlice(serverNodes), " ") + " " + strings.Join(VagrantSlice(agentNodes), " ") + " " + strings.Join(VagrantSlice(windowsAgentNodes), " ")
	nodeRoles = strings.TrimSpace(nodeRoles)

	nodeBoxes := strings.Repeat(nodeOS+" ", serverCount+agentCount)
	nodeBoxes = nodeBoxes + strings.Repeat("jborean93/WindowsServer2022"+" ", windowsAgentCount)
	nodeBoxes = strings.TrimSpace(nodeBoxes)

	nodeEnvs := fmt.Sprintf(`E2E_NODE_ROLES="%s" E2E_NODE_BOXES="%s"`, nodeRoles, nodeBoxes)

	return serverNodes, agentNodes, windowsAgentNodes, nodeEnvs
}

func CreateCluster(nodeOS string, serverCount int, agentCount int) (*TestConfig, error) {

	serverNodes, agentNodes, _, nodeEnvs := genNodeEnvs(nodeOS, serverCount, agentCount, 0)

	var testOptions string
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "E2E_") {
			testOptions += " " + env
		}
	}

	// Bring up the first server node
	cmd := fmt.Sprintf(`%s %s vagrant up --no-tty %s &> vagrant.log`, nodeEnvs, testOptions, serverNodes[0].Name)

	fmt.Println(cmd)
	if _, err := RunCommand(cmd); err != nil {
		return nil, newNodeError(cmd, serverNodes[0], err)
	}

	// Bring up the rest of the nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	for _, node := range append(serverNodes[1:], agentNodes...) {
		cmd := fmt.Sprintf(`%s %s vagrant up --no-tty %s &>> vagrant.log`, nodeEnvs, testOptions, node.Name)
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
		return nil, err
	}

	tc := &TestConfig{
		KubeconfigFile: "",
		Servers:        serverNodes,
		Agents:         agentNodes,
	}
	return tc, nil
}

func CreateMixedCluster(nodeOS string, serverCount, linuxAgentCount, windowsAgentCount int) (*TestConfig, error) {
	serverNodes, linuxAgents, windowsAgents, nodeEnvs := genNodeEnvs(nodeOS, serverCount, linuxAgentCount, windowsAgentCount)

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
		return nil, err
	}
	tc := &TestConfig{
		KubeconfigFile: "",
		Servers:        serverNodes,
		Agents:         linuxAgents,
		WindowsAgents:  windowsAgents,
	}
	return tc, nil
}

func scpRKE2Artifacts(nodes []VagrantNode) error {
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
	for _, node := range nodes {
		for _, artifact := range append(binary, images...) {
			cmd := fmt.Sprintf(`vagrant scp ../../../%s %s:/tmp/`, artifact, node.Name)
			if _, err := RunCommand(cmd); err != nil {
				return err
			}
		}
		if _, err := node.RunCmdOnNode("mkdir -p /var/lib/rancher/rke2/agent/images"); err != nil {
			return err
		}
		for _, image := range images {
			cmd := fmt.Sprintf("mv /tmp/%s /var/lib/rancher/rke2/agent/images/", filepath.Base(image))
			if _, err := node.RunCmdOnNode(cmd); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateLocalCluster creates a cluster using the locally built RKE2 bundled binary and images.
// Run at a minimum "make package-bundle" and "make package-image-runtime" first
// The vagrant-scp plugin must be installed for this function to work.
func CreateLocalCluster(nodeOS string, serverCount, agentCount int) (*TestConfig, error) {

	serverNodes, agentNodes, _, nodeEnvs := genNodeEnvs(nodeOS, serverCount, agentCount, 0)

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
		return nil, newNodeError(cmd, serverNodes[0], err)
	}

	if err := scpRKE2Artifacts(append(serverNodes, agentNodes...)); err != nil {
		return nil, err
	}
	// Install RKE2 on all nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	for _, node := range append(serverNodes, agentNodes...) {
		cmd = fmt.Sprintf(`%s %s vagrant provision %s &>> vagrant.log`, nodeEnvs, testOptions, node.Name)
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
		return nil, err
	}

	tc := &TestConfig{
		KubeconfigFile: "",
		Servers:        serverNodes,
		Agents:         agentNodes,
	}
	return tc, nil
}

func scpWindowsRKE2Artifacts(nodes []VagrantNode) error {
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
	for _, node := range nodes {
		if node.Type != Windows {
			return fmt.Errorf("node %s is not a windows node", node.Name)
		}
		for _, artifact := range append(binary, images...) {
			cmd := fmt.Sprintf(`vagrant scp ../../../%s %s:/temp/`, artifact, node.Name)
			if _, err := RunCommand(cmd); err != nil {
				return err
			}
		}
		if _, err := node.runCmdOnWindowsNode(`mv C:\temp\sha256sum-windows-amd64.txt C:\temp\sha256sum-amd64.txt`); err != nil {
			return err
		}
		if _, err := node.runCmdOnWindowsNode(`mkdir C:\var\lib\rancher\rke2\agent\images`); err != nil {
			return err
		}

		for _, image := range images {
			cmd := fmt.Sprintf("mv C:\\temp\\%s C:\\var\\lib\\rancher\\rke2\\agent\\images\\ ", filepath.Base(image))
			if _, err := node.runCmdOnWindowsNode(cmd); err != nil {
				return err
			}
		}
		if _, err := node.runCmdOnWindowsNode(`mkdir C:\usr\local\bin`); err != nil {
			return err
		}
		if _, err := node.runCmdOnWindowsNode(`mv C:\temp\rke2.exe C:\usr\local\bin\rke2.exe`); err != nil {
			return err
		}
	}
	return nil
}

func CreateLocalMixedCluster(nodeOS string, serverCount, linuxAgentCount, windowsAgentCount int) (*TestConfig, error) {
	serverNodes, linuxAgents, windowsAgents, nodeEnvs := genNodeEnvs(nodeOS, serverCount, linuxAgentCount, windowsAgentCount)

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
		return nil, err
	}

	if err := scpRKE2Artifacts(append(serverNodes, linuxAgents...)); err != nil {
		return nil, err
	}
	if err := scpWindowsRKE2Artifacts(windowsAgents); err != nil {
		return nil, err
	}
	// Install RKE2 on all nodes in parallel
	errg, _ := errgroup.WithContext(context.Background())
	allNodes := append(serverNodes, linuxAgents...)
	allNodes = append(allNodes, windowsAgents...)
	for _, node := range allNodes {
		cmd = fmt.Sprintf(`%s %s vagrant provision %s &>> vagrant.log`, nodeEnvs, testOptions, node.Name)
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
		return nil, err
	}
	tc := &TestConfig{
		KubeconfigFile: "",
		Servers:        serverNodes,
		Agents:         linuxAgents,
		WindowsAgents:  windowsAgents,
	}
	return tc, nil
}

func (config TestConfig) DeployWorkload(workload string) (string, error) {
	resourceDir := "../resource_files"
	files, err := os.ReadDir(resourceDir)
	if err != nil {
		fmt.Println("Unable to read resource manifest file for ", workload)
	}
	fmt.Println("\nDeploying", workload)
	for _, f := range files {
		filename := filepath.Join(resourceDir, f.Name())
		if strings.TrimSpace(f.Name()) == workload {
			cmd := "kubectl apply -f " + filename + " --kubeconfig=" + config.KubeconfigFile
			return RunCommand(cmd)
		}
	}
	return "", nil
}

// RestartCluster restarts the rke2 service on each server-agent given
func RestartCluster(nodes []VagrantNode) error {
	for _, node := range nodes {
		const cmd = "systemctl restart rke2-*"
		if _, err := node.RunCmdOnNode(cmd); err != nil {
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

func (v VagrantNode) FetchNodeExternalIP() (string, error) {
	cmd := "vagrant ssh " + v.Name + " -c  \"ip -f inet addr show eth1| awk '/inet / {print $2}'|cut -d/ -f1\""
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
		nodeJournal, _ = nodeErr.Node.RunCmdOnNode("sudo journalctl -u rke2* --no-pager")
		nodeJournal = "\nNode Journal Logs:\n" + nodeJournal
		if !strings.Contains(nodeErr.Node.Name, "windows-agent") {
			paths := []string{"/var/lib/rancher/rke2/agent/logs/kubelet.log", "/var/lib/rancher/rke2/agent/containerd/containerd.log"}
			for _, path := range paths {
				out, _ := nodeErr.Node.RunCmdOnNode("sudo cat " + path)
				nodeJournal += "\n" + path + ":\n" + out + "\n"
			}
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

func GenKubeConfigFile(server VagrantNode) (string, error) {
	kubeConfig, err := server.RunCmdOnNode("cat /etc/rancher/rke2/rke2.yaml")
	if err != nil {
		return "", err
	}

	nodeIP, err := server.FetchNodeExternalIP()
	if err != nil {
		return "", err
	}
	kubeConfig = strings.Replace(kubeConfig, "127.0.0.1", nodeIP, 1)
	kubeConfigFile := fmt.Sprintf("kubeconfig-%s", server.Name)
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
func (v VagrantNode) RunCmdOnNode(cmd string) (string, error) {
	switch v.Type {
	case Linux:
		return v.runCmdOnLinuxNode(cmd)
	case Windows:
		return v.runCmdOnWindowsNode(cmd)
	default:
		return "", fmt.Errorf("unknown node type: %d", v.Type)
	}
}

// RunCmdOnNode executes a command from within the given node as sudo
func (v VagrantNode) runCmdOnLinuxNode(cmd string) (string, error) {
	runcmd := "vagrant ssh -c \"sudo " + cmd + "\" " + v.Name
	out, err := RunCommand(runcmd)
	// On GHA CI we see warnings about "[fog][WARNING] Unrecognized arguments: libvirt_ip_command"
	// these are added to the command output and need to be removed
	out = strings.ReplaceAll(out, "[fog][WARNING] Unrecognized arguments: libvirt_ip_command\n", "")
	if err != nil {
		return out, fmt.Errorf("failed to run command: %s on node %s: %s, %v", cmd, v.Name, out, err)
	}
	return out, nil
}

// RunCmdOnWindowsNode executes a command from within the given windows node
func (v VagrantNode) runCmdOnWindowsNode(cmd string) (string, error) {
	runcmd := "vagrant ssh -c 'powershell.exe -Command \"" + cmd + "\"' " + v.Name
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
func StartCluster(nodes []VagrantNode) error {
	for _, node := range nodes {
		cmd := "systemctl start rke2"
		if strings.Contains(node.Name, "server") {
			cmd += "-server"
		}
		if strings.Contains(node.Name, "agent") {
			cmd += "-agent"
		}
		if _, err := node.RunCmdOnNode(cmd); err != nil {
			return err
		}
	}
	return nil
}

// StopCluster starts the rke2 service on each node given
func StopCluster(nodes []VagrantNode) error {
	for _, node := range nodes {
		cmd := "systemctl stop rke2*"
		if _, err := node.RunCmdOnNode(cmd); err != nil {
			return err
		}
	}
	return nil
}

func UpgradeCluster(servers []VagrantNode, agents []VagrantNode) error {
	for _, server := range servers {
		cmd := "E2E_RELEASE_CHANNEL=commit vagrant provision " + server.Name
		fmt.Println(cmd)
		if out, err := RunCommand(cmd); err != nil {
			fmt.Println("Error Upgrading Cluster", out)
			return err
		}
	}
	for _, agent := range agents {
		cmd := "E2E_RELEASE_CHANNEL=commit vagrant provision " + agent.Name
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
