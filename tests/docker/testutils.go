package docker

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type TestConfig struct {
	TestDir        string
	KubeconfigFile string
	Token          string
	Servers        []DockerNode
	Agents         []DockerNode
	ServerYaml     string
	AgentYaml      string
	DualStack      bool // If true, the docker containers will be attached to a dual-stack network
}

type DockerNode struct {
	Name string
	IP   string
	Port int    // Not filled by agent nodes
	URL  string // Not filled by agent nodes
}

// NewTestConfig initializes the test environment and returns the test config
// A random token is generated for the cluster
func NewTestConfig() (*TestConfig, error) {
	config := &TestConfig{}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "rke2-test-")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}
	config.TestDir = tempDir

	// Create required directories
	if err := os.MkdirAll(filepath.Join(config.TestDir, "logs"), 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %v", err)
	}

	// Generate random secret
	config.Token = fmt.Sprintf("%012d", rand.Int63n(1000000000000))
	return config, nil
}

// portFree checks if a port is in use and returns true if it is free
func portFree(port int) bool {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// getPort finds an available port
func getPort() int {
	var port int
	for i := 0; i < 100; i++ {
		port = 10000 + rand.Intn(50000)
		if portFree(port) {
			return port
		}
	}
	return -1
}

// ProvisionServers starts the required number of containers and
// installs RKE2 as a service on each of them.
func (config *TestConfig) ProvisionServers(numOfServers int) error {
	for i := 0; i < numOfServers; i++ {

		// If a server i already exists, skip. This is useful for scenarios where
		// the first server is started separate from the rest of the servers
		if config.Servers != nil && i < len(config.Servers) {
			continue
		}

		testID := filepath.Base(config.TestDir)
		name := fmt.Sprintf("server-%d-%s", i, strings.ToLower(testID))
		port := getPort()
		if port == -1 {
			return fmt.Errorf("failed to find an available port")
		}

		var joinServer string
		if i != 0 {
			if config.Servers[0].URL == "" {
				return fmt.Errorf("first server URL is empty")
			}
			joinServer = fmt.Sprintf("-e RKE2_URL=%s", config.Servers[0].URL)
		}
		newServer := DockerNode{
			Name: name,
			Port: port,
		}

		// Generate sha256sum file if it doesn't exist
		if _, err := os.Stat("../../../dist/artifacts/sha256sum-amd64.txt"); os.IsNotExist(err) {
			if _, err := RunCommand("sha256sum ../../../dist/artifacts/rke2.linux-amd64.tar.gz > ../../../dist/artifacts/sha256sum-amd64.txt"); err != nil {
				return fmt.Errorf("failed to generate sha256sum file: %v", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to check sha256sum file: %v", err)
		}

		dualStackConfig := ""
		if config.DualStack {
			// Check if the docker network exists, if not create it
			networkName := "rke2-test-dualstack"
			if _, err := RunCommand(fmt.Sprintf("docker network inspect %s", networkName)); err != nil {
				cmd := fmt.Sprintf("docker network create --ipv6 --subnet=fd11:decf:c0ff:ee::/64 %s", networkName)
				if _, err := RunCommand(cmd); err != nil {
					return fmt.Errorf("failed to create dual-stack network: %v", err)
				}
			}
			dualStackConfig = "--network rke2-test-dualstack"
		}

		dRun := strings.Join([]string{"docker run -d",
			"--name", name,
			"--hostname", name,
			"--privileged",
			"-p", fmt.Sprintf("127.0.0.1:%d:6443", port),
			"--memory", "3072m",
			"-e", fmt.Sprintf("RKE2_TOKEN=%s", config.Token),
			joinServer,
			dualStackConfig,
			"-e", "RKE2_DEBUG=true",
			"-e", "GOCOVERDIR=/tmp/rke2-cov",
			"-e", "PATH=$PATH:/var/lib/rancher/rke2/bin",
			"-v", "/sys/fs/bpf:/sys/fs/bpf",
			"-v", "/lib/modules:/lib/modules",
			"-v", "/sys/fs/cgroup:/run/cilium/cgroupv2",
			"-v", "/var/run/docker.sock:/var/run/docker.sock",
			"-v", "/var/lib/docker:/var/lib/docker",
			"--mount", "type=bind,source=$(pwd)/../../../dist/artifacts/rke2.linux-amd64.tar.gz,target=/tmp/rke2-artifacts/rke2.linux-amd64.tar.gz",
			"--mount", "type=bind,source=$(pwd)/../../../dist/artifacts/sha256sum-amd64.txt,target=/tmp/rke2-artifacts/sha256sum-amd64.txt",
			"--mount", "type=bind,source=$(pwd)/../../../build/images/rke2-images.linux-amd64.tar.zst,target=/var/lib/rancher/rke2/agent/images/rke2-images.linux-amd64.tar.zst",
			"rancher/systemd-node:v0.0.5",
			"/usr/lib/systemd/systemd --unit=noop.target --show-status=true"}, " ")
		if out, err := RunCommand(dRun); err != nil {
			return fmt.Errorf("failed to start systemd container: %s: %v", out, err)
		}
		time.Sleep(5 * time.Second)
		cmd := "mkdir -p /tmp/rke2-cov"
		if out, err := newServer.RunCmdOnNode(cmd); err != nil {
			return fmt.Errorf("failed to create coverage directory: %s: %v", out, err)
		}

		// Create empty config.yaml for later use
		cmd = "mkdir -p /etc/rancher/rke2; touch /etc/rancher/rke2/config.yaml"
		if out, err := newServer.RunCmdOnNode(cmd); err != nil {
			return fmt.Errorf("failed to create empty config.yaml: %s: %v", out, err)
		}
		// Write the raw YAML directly to the config.yaml on the systemd-node container
		if config.ServerYaml != "" {
			cmd = fmt.Sprintf("echo '%s' > /etc/rancher/rke2/config.yaml", config.ServerYaml)
			if out, err := newServer.RunCmdOnNode(cmd); err != nil {
				return fmt.Errorf("failed to write server yaml: %s: %v", out, err)
			}
		}

		if _, err := newServer.RunCmdOnNode("curl -sfL https://get.rke2.io | INSTALL_RKE2_ARTIFACT_PATH=/tmp/rke2-artifacts sh -"); err != nil {
			return fmt.Errorf("failed to install server: %v", err)
		}

		if _, err := newServer.RunCmdOnNode("systemctl enable rke2-server"); err != nil {
			return fmt.Errorf("failed to enable server: %v", err)
		}

		// Fill RKE2_* environment variables.
		envVars, err := newServer.RunCmdOnNode("env | grep ^RKE2_")
		if err != nil {
			return fmt.Errorf("failed to get RKE2_* environment variables: %v", err)
		}
		envFile := strings.ReplaceAll(envVars, "\n", "\\n")
		writeCmd := fmt.Sprintf("printf '%s' > /usr/local/lib/systemd/system/rke2-server.env", envFile)
		if _, err := newServer.RunCmdOnNode(writeCmd); err != nil {
			return fmt.Errorf("failed to write env vars to /usr/local/lib/systemd/system/rke2-server.env: %v", err)
		}

		// Get the IP address of the container
		if config.DualStack {
			cmd = "docker inspect --format '{{range $k,$v := .NetworkSettings.Networks}}{{printf \"%s\" $v.IPAddress}}{{end}}' " + name
		} else {
			cmd = "docker inspect --format '{{ .NetworkSettings.IPAddress }}' " + name
		}
		ipOutput, err := RunCommand(cmd)
		if err != nil {
			return err
		}
		ip := strings.TrimSpace(ipOutput)

		url := fmt.Sprintf("https://%s:9345", ip)
		newServer.URL = url
		newServer.IP = ip
		config.Servers = append(config.Servers, newServer)

		fmt.Printf("Started %s @ %s\n", name, url)

		// Sleep for a bit to allow the first server to start
		if i == 0 && numOfServers > 1 {
			time.Sleep(10 * time.Second)
		}
	}
	return nil
}

func (config *TestConfig) ProvisionAgents(numOfAgents int) error {
	testID := filepath.Base(config.TestDir)

	var g errgroup.Group
	for i := 0; i < numOfAgents; i++ {
		i := i // capture loop variable
		g.Go(func() error {
			name := fmt.Sprintf("agent-%d-%s", i, strings.ToLower(testID))

			newAgent := DockerNode{
				Name: name,
			}

			dualStackConfig := ""
			if config.DualStack {
				dualStackConfig = "--network rke2-test-dualstack"
			}

			dRun := strings.Join([]string{"docker run -d",
				"--name", name,
				"--hostname", name,
				"--privileged",
				"--memory", "2048m",
				"-e", fmt.Sprintf("RKE2_TOKEN=%s", config.Token),
				"-e", fmt.Sprintf("RKE2_URL=%s", config.Servers[0].URL),
				dualStackConfig,
				"-e", "RKE2_DEBUG=true",
				"-e", "GOCOVERDIR=/tmp/rke2-cov",
				"-v", "/sys/fs/bpf:/sys/fs/bpf",
				"-v", "/lib/modules:/lib/modules",
				"-v", "/sys/fs/cgroup:/run/cilium/cgroupv2",
				"-v", "/var/run/docker.sock:/var/run/docker.sock",
				"-v", "/var/lib/docker:/var/lib/docker",
				"--mount", "type=bind,source=$(pwd)/../../../dist/artifacts/rke2.linux-amd64.tar.gz,target=/tmp/rke2-artifacts/rke2.linux-amd64.tar.gz",
				"--mount", "type=bind,source=$(pwd)/../../../dist/artifacts/sha256sum-amd64.txt,target=/tmp/rke2-artifacts/sha256sum-amd64.txt",
				"--mount", "type=bind,source=$(pwd)/../../../build/images/rke2-images.linux-amd64.tar.zst,target=/var/lib/rancher/rke2/agent/images/rke2-images.linux-amd64.tar.zst",
				"rancher/systemd-node:v0.0.5",
				"/usr/lib/systemd/systemd --unit=noop.target --show-status=true"}, " ")
			if out, err := RunCommand(dRun); err != nil {
				return fmt.Errorf("failed to start systemd container: %s: %v", out, err)
			}
			time.Sleep(5 * time.Second)

			// Create empty config.yaml for later use
			cmd := "mkdir -p /etc/rancher/rke2; touch /etc/rancher/rke2/config.yaml"
			if out, err := newAgent.RunCmdOnNode(cmd); err != nil {
				return fmt.Errorf("failed to create empty config.yaml: %s: %v", out, err)
			}
			// Write the raw YAML directly to the config.yaml on the systemd-node container
			if config.AgentYaml != "" {
				cmd = fmt.Sprintf("echo '%s' > /etc/rancher/rke2/config.yaml", config.AgentYaml)
				if out, err := newAgent.RunCmdOnNode(cmd); err != nil {
					return fmt.Errorf("failed to write server yaml: %s: %v", out, err)
				}
			}

			if _, err := newAgent.RunCmdOnNode("curl -sfL https://get.rke2.io | INSTALL_RKE_TYPE='agent' INSTALL_RKE2_ARTIFACT_PATH=/tmp/rke2-artifacts sh -"); err != nil {
				return fmt.Errorf("failed to install agent: %v", err)
			}

			// Fill RKE2_* environment variables.
			envVars, err := newAgent.RunCmdOnNode("env | grep ^RKE2_")
			if err != nil {
				return fmt.Errorf("failed to get RKE2_* environment variables: %v", err)
			}
			envFile := strings.ReplaceAll(envVars, "\n", "\\n")
			writeCmd := fmt.Sprintf("printf '%s' > /usr/local/lib/systemd/system/rke2-agent.env", envFile)
			if _, err := newAgent.RunCmdOnNode(writeCmd); err != nil {
				return fmt.Errorf("failed to write env vars to /usr/local/lib/systemd/system/rke2-agent.env: %v", err)
			}

			if _, err := newAgent.RunCmdOnNode("systemctl enable rke2-agent"); err != nil {
				return fmt.Errorf("failed to enable agent: %v", err)
			}

			// Get the IP address of the container
			if config.DualStack {
				cmd = "docker inspect --format '{{range $k,$v := .NetworkSettings.Networks}}{{printf \"%s\" $v.IPAddress}}{{end}}' " + name
			} else {
				cmd = "docker inspect --format '{{ .NetworkSettings.IPAddress }}' " + name
			}
			ipOutput, err := RunCommand(cmd)
			if err != nil {
				return err
			}
			ip := strings.TrimSpace(ipOutput)
			newAgent.IP = ip
			config.Agents = append(config.Agents, newAgent)

			fmt.Printf("Started %s @ %s\n", newAgent.Name, newAgent.IP)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

func (config *TestConfig) RemoveNode(nodeName string) error {
	cmd := fmt.Sprintf("docker stop %s", nodeName)
	if _, err := RunCommand(cmd); err != nil {
		return fmt.Errorf("failed to stop node %s: %v", nodeName, err)
	}
	cmd = fmt.Sprintf("docker rm %s", nodeName)
	if _, err := RunCommand(cmd); err != nil {
		return fmt.Errorf("failed to remove node %s: %v", nodeName, err)
	}
	return nil
}

// Returns a list of all server names
func (config *TestConfig) GetServerNames() []string {
	var serverNames []string
	for _, server := range config.Servers {
		serverNames = append(serverNames, server.Name)
	}
	return serverNames
}

// Returns a list of all agent names
func (config *TestConfig) GetAgentNames() []string {
	var agentNames []string
	for _, agent := range config.Agents {
		agentNames = append(agentNames, agent.Name)
	}
	return agentNames
}

// Returns a list of all node names
func (config *TestConfig) GetNodeNames() []string {
	var nodeNames []string
	nodeNames = append(nodeNames, config.GetServerNames()...)
	nodeNames = append(nodeNames, config.GetAgentNames()...)
	return nodeNames
}

func (config *TestConfig) Cleanup() error {

	errs := make([]error, 0)
	// Stop and remove all servers
	for _, server := range config.Servers {
		if err := config.RemoveNode(server.Name); err != nil {
			errs = append(errs, err)
		}
	}
	config.Servers = nil

	// Stop and remove all agents
	for _, agent := range config.Agents {
		if err := config.RemoveNode(agent.Name); err != nil {
			errs = append(errs, err)
		}
	}
	config.Agents = nil

	// Remove volumes created by the agent/server containers
	cmd := fmt.Sprintf("docker volume ls -q | grep -F %s | xargs -r docker volume rm", strings.ToLower(filepath.Base(config.TestDir)))
	if _, err := RunCommand(cmd); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove volumes: %v", err))
	}

	// Remove dual-stack network if it exists
	if config.DualStack {
		if _, err := RunCommand("docker network rm rke2-test-dualstack"); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove dual-stack network: %v", err))
		}
	}

	// Error out if we hit any issues
	if len(errs) > 0 {
		return fmt.Errorf("cleanup failed: %v", errs)
	}

	if config.TestDir != "" {
		return os.RemoveAll(config.TestDir)
	}
	return nil
}

// CopyAndModifyKubeconfig copies out kubeconfig from first control-plane server
// and updates the port to match the external port
func (config *TestConfig) CopyAndModifyKubeconfig() error {
	if len(config.Servers) == 0 {
		return fmt.Errorf("no servers available to copy kubeconfig")
	}

	serverID := 0
	// Check the config.yaml of each server to find the first server that has the apiserver enabled
	for i, node := range config.Servers {
		out, err := node.RunCmdOnNode("cat /etc/rancher/rke2/config.yaml")
		if err != nil {
			return fmt.Errorf("failed to get config.yaml: %v", err)
		}
		if !strings.Contains(out, "disable-apiserver: true") {
			serverID = i
			break
		}
	}

	cmd := fmt.Sprintf("docker cp %s:/etc/rancher/rke2/rke2.yaml %s/kubeconfig.yaml", config.Servers[serverID].Name, config.TestDir)
	if _, err := RunCommand(cmd); err != nil {
		return fmt.Errorf("failed to copy kubeconfig: %v", err)
	}

	cmd = fmt.Sprintf("sed -i -e \"s/:6443/:%d/g\" %s/kubeconfig.yaml", config.Servers[serverID].Port, config.TestDir)
	if _, err := RunCommand(cmd); err != nil {
		return fmt.Errorf("failed to update kubeconfig: %v", err)
	}
	config.KubeconfigFile = filepath.Join(config.TestDir, "kubeconfig.yaml")
	fmt.Println("Kubeconfig file: ", config.KubeconfigFile)
	return nil
}

// RunCmdOnNode runs a command on a docker container
func (node DockerNode) RunCmdOnNode(cmd string) (string, error) {
	dCmd := fmt.Sprintf("docker exec %s /bin/sh -c \"%s\"", node.Name, cmd)
	out, err := RunCommand(dCmd)
	if err != nil {
		return out, fmt.Errorf("%v: on node %s: %s", err, node.Name, out)
	}
	return out, nil
}

// RunCommand Runs command on the host.
func RunCommand(cmd string) (string, error) {
	c := exec.Command("bash", "-c", cmd)
	out, err := c.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("failed to run command: %s, %v", cmd, err)
	}
	return string(out), err
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (config *TestConfig) DumpResources() string {
	cmd := "kubectl get pod,node -A -o wide --kubeconfig=" + config.KubeconfigFile
	out, err := RunCommand(cmd)
	if err != nil {
		return fmt.Sprintf("Failed to run command %q: %v", cmd, err)
	}
	return out
}

// Dump pod logs for all nodes
func (config *TestConfig) DumpPodLogs(lines int) string {
	logs := &strings.Builder{}
	cmd := fmt.Sprintf("tail -n %d /var/log/pods/*/*/*", lines)
	for _, node := range append(config.Servers, config.Agents...) {
		if l, err := node.RunCmdOnNode(cmd); err != nil {
			fmt.Fprintf(logs, "** failed to tail pod logs for node %s ***\n%v\n", node.Name, err)
		} else {
			fmt.Fprintf(logs, "** pod logs for node %s ***\n%s\n", node.Name, l)
		}
	}
	return logs.String()
}

// Dump kubelet and containerd logs for all nodes
func (config *TestConfig) DumpComponentLogs(lines int) string {
	logs := &strings.Builder{}
	cmd := fmt.Sprintf("tail -n %d /var/lib/rancher/rke2/agent/containerd/containerd.log /var/lib/rancher/rke2/agent/logs/kubelet.log", lines)
	for _, node := range append(config.Servers, config.Agents...) {
		if l, err := node.RunCmdOnNode(cmd); err != nil {
			fmt.Fprintf(logs, "** failed to tail component logs for node %s ***\n%v\n", node.Name, err)
		} else {
			fmt.Fprintf(logs, "** component logs for node %s ***\n%s\n", node.Name, l)
		}
	}
	return logs.String()
}

// Dump journactl logs for all nodes
func (config *TestConfig) DumpServiceLogs(lines int) string {
	logs := &strings.Builder{}
	for _, node := range append(config.Servers, config.Agents...) {
		if l, err := node.DumpServiceLogs(lines); err != nil {
			fmt.Fprintf(logs, "** failed to read journald log for node %s ***\n%v\n", node.Name, err)
		} else {
			fmt.Fprintf(logs, "** journald log for node %s ***\n%s\n", node.Name, l)
		}
	}
	return logs.String()
}

// Dump the journalctl logs for the rke2 service
func (node DockerNode) DumpServiceLogs(lines int) (string, error) {
	var cmd string
	if strings.Contains(node.Name, "agent") {
		cmd = fmt.Sprintf("journalctl -u rke2-agent -n %d", lines)
	} else {
		cmd = fmt.Sprintf("journalctl -u rke2-server -n %d", lines)
	}
	res, err := node.RunCmdOnNode(cmd)
	if strings.Contains(res, "No entries") {
		return "", fmt.Errorf("no logs found")
	}
	return res, err
}

// TODO the below functions are replicated from e2e test utils. Consider combining into common package
func (config TestConfig) DeployWorkload(workload string) (string, error) {
	resourceDir := "../resources"
	files, err := os.ReadDir(resourceDir)
	if err != nil {
		err = fmt.Errorf("%s : Unable to read resource manifest file for %s", err, workload)
		return "", err
	}
	if _, err = os.Stat(filepath.Join(resourceDir, workload)); err != nil {
		return "", err
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

func FetchClusterIP(kubeconfig string, servicename string) (string, error) {
	cmd := "kubectl get svc " + servicename + " -o jsonpath='{.spec.clusterIP}' --kubeconfig=" + kubeconfig
	return RunCommand(cmd)
}

// RestartCluster restarts the RKE2 service on each node given
func RestartCluster(nodes []DockerNode) error {
	var cmd string
	for _, node := range nodes {
		if strings.Contains(node.Name, "agent") {
			cmd = "systemctl restart rke2-agent"
		} else {
			cmd = "systemctl restart rke2-server"
		}
		if _, err := node.RunCmdOnNode(cmd); err != nil {
			logs, _ := node.DumpServiceLogs(10)
			return fmt.Errorf("journal logs: %s: %v", logs, err)
		}
	}
	return nil
}
