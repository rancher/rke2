package util

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rancher/rke2/tests/terraform/core/rke2error"
	"golang.org/x/crypto/ssh"
)

// RunCommandOnNode executes a command from within the given node
func RunCommandOnNode(cmd string, serverIP string, sshUser string, sshKey string) (string, error) {
	server := serverIP + ":22"
	conn, err := configureSSH(server, sshUser, sshKey)
	if err != nil {
		return "", rke2error.NewRke2Error(cmd, conn, "something wrong on:", err)
	}
	res, err := runsshCommand(cmd, conn)
	if err != nil {
		return "", rke2error.NewRke2Error(cmd, res, "something wrong on:", err)
	}
	res = strings.TrimSpace(res)

	return res, nil
}

// RunCommandHost executes a command on the host
func RunCommandHost(cmd ...string) (string, error) {
	var output string
	for _, cmd := range cmd {
		c := exec.Command("bash", "-c", cmd)
		out, err := c.CombinedOutput()
		output += string(out)
		if err != nil {
			err = rke2error.NewRke2Error(cmd, output, "something wrong on:", err)
			return "", err
		}
	}

	return output, nil
}

// Basepath returns the base path of the project
func Basepath() string {
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(b), "../..")
}

// PrintFileContents prints the contents of the file as [] string
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
	var config *ssh.ClientConfig

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
