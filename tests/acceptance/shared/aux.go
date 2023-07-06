package shared

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh"
)

// RunCommandHost executes a command on the host
func RunCommandHost(cmds ...string) (string, error) {
	if cmds == nil {
		return "", fmt.Errorf("cmd should not be empty")
	}

	var output, errOut bytes.Buffer
	for _, cmd := range cmds {
		c := exec.Command("bash", "-c", cmd)
		c.Stdout = &output
		c.Stderr = &errOut
		err := c.Run()
		if err != nil {
			fmt.Println(errOut.String())
			return output.String(), fmt.Errorf("executing command: %s: %w", cmd, err)
		}
	}

	return output.String(), nil
}

// RunCommandOnNode executes a command on the node SSH
func RunCommandOnNode(cmd string, ServerIP string) (string, error) {
	if cmd == "" {
		return "", fmt.Errorf("cmd should not be empty")
	}

	host := ServerIP + ":22"
	conn, err := configureSSH(host)
	if err != nil {
		return fmt.Errorf("failed to configure SSH: %v", err).Error(), err
	}

	stdout, stderr, err := runsshCommand(cmd, conn)
	if err != nil {
		return fmt.Errorf("\ncommand: %s \n failed with error: %v", cmd, err).Error(), err
	}

	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)

	if stderr != "" && (!strings.Contains(stderr, "error") ||
		!strings.Contains(stderr, "exit status 1") ||
		!strings.Contains(stderr, "exit status 2")) {
		return stderr, nil
	} else if stderr != "" {
		return fmt.Errorf("\ncommand: %s \n failed with error: %v", cmd, stderr).Error(), err
	}

	return stdout, err
}

// BasePath returns the base path of the project.
func BasePath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(b), "../..")
}

// PrintFileContents prints the contents of the file as [] string.
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

// CountOfStringInSlice Used to count the pods using prefix passed in the list of pods.
func CountOfStringInSlice(str string, pods []Pod) int {
	var count int
	for _, p := range pods {
		if strings.Contains(p.Name, str) {
			count++
		}
	}
	return count
}

// GetRke2Version returns the rke2 version with commit hash
func GetRke2Version() string {
	ips := FetchNodeExternalIP()
	for _, ip := range ips {
		res, err := RunCommandOnNode("rke2 --version", ip)
		if err != nil {
			return err.Error()
		}
		return res
	}

	return ""
}

// AddHelmRepo adds a helm repo to the cluster.
func AddHelmRepo(name, url string) (string, error) {
	InstallHelm := "curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash"
	addRepo := fmt.Sprintf("helm repo add %s %s", name, url)
	update := "helm repo update"
	installRepo := fmt.Sprintf("helm install %s %s/%s -n kube-system --kubeconfig=%s",
		name, name, name, KubeConfigFile)

	nodeExternalIP := FetchNodeExternalIP()
	for _, ip := range nodeExternalIP {
		_, err := RunCommandOnNode(InstallHelm, ip)
		if err != nil {
			return "", err
		}
	}

	return RunCommandHost(addRepo, update, installRepo)
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

func configureSSH(host string) (*ssh.Client, error) {
	var config *ssh.ClientConfig

	authMethod, err := publicKey(AccessKey)
	if err != nil {
		return nil, err
	}
	config = &ssh.ClientConfig{
		User: AwsUser,
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

func runsshCommand(cmd string, conn *ssh.Client) (string, string, error) {
	session, err := conn.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	errssh := session.Run(cmd)
	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if errssh != nil {
		return stdoutStr, stderrStr, fmt.Errorf("error on command execution: %v", errssh)
	}

	return stdoutStr, stderrStr, nil
}

// JoinCommands joins the first command with some arg
func JoinCommands(cmd, kubeconfigFlag string) string {
	cmds := strings.Split(cmd, ";")
	joinedCmd := cmds[0] + kubeconfigFlag

	if len(cmds) > 1 {
		secondCmd := strings.Join(cmds[1:], ",")
		joinedCmd += " " + secondCmd
	}

	return joinedCmd
}
