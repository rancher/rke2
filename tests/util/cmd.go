package util

import (
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

func findRke2Executable() string {
	rke2Bin := "bin/rke2"
	for {
		_, err := os.Stat(rke2Bin)
		if err != nil {
			rke2Bin = "../" + rke2Bin
			continue
		}
		break
	}
	return rke2Bin
}

// Rke2Cmd launches the provided Rke2 command via exec. Command blocks until finished.
// Kubectl commands can also be passed.
// Command output from both Stderr and Stdout is provided via string.
//   cmdEx1, err := Rke2Cmd("etcd-snapshot", "ls")
//   cmdEx2, err := Rke2Cmd("kubectl", "get", "pods", "-A")
func Rke2Cmd(cmdName string, cmdArgs ...string) (string, error) {
	if cmdName == "kubectl" {
		byteOut, err := exec.Command(cmdName, cmdArgs...).CombinedOutput()
		return string(byteOut), err
	}
	rke2Bin := findRke2Executable()
	rke2Cmd := append([]string{cmdName}, cmdArgs...)
	byteOut, err := exec.Command(rke2Bin, rke2Cmd...).CombinedOutput()
	return string(byteOut), err
}

func Rke2Ready() bool {
	podsToCheck := []string{
		"etcd-rke2-server",
		"kube-apiserver-rke2-server",
		"kube-proxy-rke2-server",
		"kube-scheduler-rke2-server",
		"rke2-coredns-rke2-coredns",
	}
	pods, err := Rke2Cmd("kubectl", "get", "pods", "-A")
	if err != nil {
		return false
	}
	for _, pod := range podsToCheck {
		reg := pod + ".+Running"
		match, err := regexp.MatchString(reg, pods)
		if !match || err != nil {
			logrus.Error(err)
			return false
		}
	}
	return true
}

func contains(source []string, target string) bool {
	for _, s := range source {
		if s == target {
			return true
		}
	}
	return false
}

// ServerArgsPresent checks if the given arguments are found in the running k3s server
func ServerArgsPresent(neededArgs []string) bool {
	currentArgs, err := Rke2ServerArgs()
	if err != nil {
		logrus.Error(err)
		return false
	}
	for _, arg := range neededArgs {
		if !contains(currentArgs, arg) {
			return false
		}
	}
	return true
}

// Rke2ServerArgs returns the list of arguments that the rke2 server launched with
func Rke2ServerArgs() ([]string, error) {
	results, err := Rke2Cmd("kubectl", "get", "nodes", "-o", `jsonpath='{.items[0].metadata.annotations.rke2\.io/node-args}'`)
	if err != nil {
		return nil, err
	}
	res := strings.ReplaceAll(results, "'", "")
	var args []string
	if err := json.Unmarshal([]byte(res), &args); err != nil {
		return nil, err
	}
	return args, nil
}
