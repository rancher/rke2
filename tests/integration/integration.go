package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/flock"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const lockFile = "/tmp/rke2-test.lock"

func findFile(file string) (string, error) {
	i := 0
	for {
		if i > 3 {
			return "", fmt.Errorf("could not find %s", file)
		}
		if _, err := os.Stat(file); err != nil {
			file = "../" + file
			continue
		}
		i++
		break
	}

	return file, nil
}

func findRKE2Executable() (string, error) {
	return findFile("bin/rke2")
}

func findBundleExecutable(binary string) (string, error) {
	return findFile("bundle/bin/" + binary)
}

// RKE2Cmd launches the provided RKE2 command via exec. Command blocks until finished.
// Kubectl commands can also be passed.
// Command output from both Stderr and Stdout is provided via string.
// cmdEx1, err := RKE2Cmd("etcd-snapshot", "ls")
// cmdEx2, err := RKE2Cmd("kubectl", "get", "pods", "-A")
func RKE2Cmd(cmdName string, cmdArgs ...string) (string, error) {
	if cmdName == "kubectl" {
		byteOut, err := exec.Command(cmdName, cmdArgs...).CombinedOutput()
		return string(byteOut), err
	}
	rke2Bin, err := findRKE2Executable()
	if err != nil {
		return "", err
	}
	rke2Cmd := append([]string{cmdName}, cmdArgs...)
	byteOut, err := exec.Command(rke2Bin, rke2Cmd...).CombinedOutput()
	return string(byteOut), err
}

// isRoot return true if the user is root (UID 0)
func isRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		return false
	}
	return currentUser.Uid == "0"
}

func AcquireTestLock() (int, error) {
	logrus.Info("waiting to get test lock")
	return flock.Acquire(lockFile)
}

// StartServer acquires an exclusive lock on a temporary file, then launches a RKE2 cluster
// with the provided arguments. Subsequent/parallel calls to this function will block until
// the original lock is cleared using RKE2KillServer
// A file is returned to capture the output of the RKE2 server for debugging
func StartServer(inputArgs ...string) (*os.File, error) {
	if !isRoot() {
		return nil, errors.New("integration tests must be run as sudo/root")
	}

	// Prepary RKE2 images if they are not present
	_, err := os.Stat("/var/lib/rancher/rke2/agent/images")
	if err != nil {
		os.MkdirAll("/var/lib/rancher/rke2/agent", 0755)
		imageDir, err := findFile("build/images")
		if err != nil {
			return nil, err
		}
		cmd := exec.Command("cp", "-r", imageDir, "/var/lib/rancher/rke2/agent/images")
		if res, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("error copying images: %s: %w", res, err)
		}
	}
	rke2Bin, err := findRKE2Executable()
	if err != nil {
		return nil, err
	}
	rke2Cmd := append([]string{"server"}, inputArgs...)
	cmd := exec.Command(rke2Bin, rke2Cmd...)
	// Pipe output to a file for debugging later
	f, err := os.Create("./r2log.txt")
	if err != nil {
		return nil, err
	}
	cmd.Stderr = f
	err = cmd.Start()
	return f, err
}

// KillServer terminates the running RKE2 server and its children using rke2-killall.sh
func KillServer(log *os.File) error {

	killall, err := findBundleExecutable("rke2-killall.sh")
	if err != nil {
		return err
	}
	cmd := exec.Command(killall)
	if res, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error killing rke2 server: %s: %w", res, err)
	}

	if log != nil {
		log.Close()
		os.Remove(log.Name())
	}
	time.Sleep(2 * time.Second)
	return nil
}

// SaveLog closes the server log file and optionally dumps the contents to stdout
func SaveLog(log *os.File, dump bool) error {
	log.Close()
	if !dump {
		return nil
	}
	log, err := os.Open(log.Name())
	if err != nil {
		return err
	}
	defer log.Close()
	b, err := io.ReadAll(log)
	if err != nil {
		return err
	}
	fmt.Printf("Server Log Dump:\n\n%s\n\n", b)
	return nil
}

// Cleanup unlocks the test-lock and run the rke2-uninstall.sh script
// with one exception: we save and then restore the agent/images directory,
// for use with the next test.
func Cleanup(rke2TestLock int) error {
	// Save the agent/images directory
	if err := os.MkdirAll("/tmp/images-backup", 0755); err != nil && !errors.Is(err, os.ErrExist) {
		return fmt.Errorf("failed to make backup images directory: %w", err)
	}
	cmd := exec.Command("sh", "-c", "mv /var/lib/rancher/rke2/agent/images/* /tmp/images-backup/")
	if res, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error backing up images directory: %s: %w", res, err)
	}

	uninstall, err := findBundleExecutable("rke2-uninstall.sh")
	if err != nil {
		return err
	}
	// We don't care about the return value of the uninstall script,
	// as it will always return an error because no rk2e service is running
	exec.Command(uninstall).Run()

	// Restore the agent/images directory
	if err := os.MkdirAll("/var/lib/rancher/rke2/agent", 0755); err != nil {
		return fmt.Errorf("failed to make agent directory: %w", err)
	}
	cmd = exec.Command("sh", "-c", "mv /tmp/images-backup/* /var/lib/rancher/rke2/agent/images/")
	if res, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error restoring images directory: %s: %w", res, err)
	}
	if rke2TestLock != -1 {
		return flock.Release(rke2TestLock)
	}
	return nil
}

// ServerReady checks if the server is ready by checking the status of the pods
// and deployments that are required for the server to be operational
// On success, returns nil
func ServerReady() error {
	hn, err := os.Hostname()
	if err != nil {
		return err
	}
	podsToCheck := []string{
		"etcd-" + hn,
		"kube-apiserver-" + hn,
		"kube-proxy-" + hn,
		"kube-scheduler-" + hn,
	}
	deploymentsToCheck := []string{
		"rke2-coredns-rke2-coredns",
		"rke2-metrics-server",
		"rke2-snapshot-controller",
	}

	pods, err := ParsePods("kube-system", metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pod := range podsToCheck {
		ready := false
		for _, p := range pods {
			if p.Name == pod && p.Status.Phase == "Running" {
				ready = true
			}
		}
		if !ready {
			return fmt.Errorf("pod %s is not ready", pod)
		}
	}

	return CheckDeployments(deploymentsToCheck)
}

func ParsePods(namespace string, opts metav1.ListOptions) ([]corev1.Pod, error) {
	clientSet, err := k8sClient()
	if err != nil {
		return nil, err
	}
	pods, err := clientSet.CoreV1().Pods(namespace).List(context.Background(), opts)
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

// CheckDeployments checks if the provided list of deployments are ready, otherwise returns an error
func CheckDeployments(deployments []string) error {

	deploymentSet := make(map[string]bool)
	for _, d := range deployments {
		deploymentSet[d] = false
	}

	client, err := k8sClient()
	if err != nil {
		return err
	}
	deploymentList, err := client.AppsV1().Deployments("kube-system").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, deployment := range deploymentList.Items {
		if _, ok := deploymentSet[deployment.Name]; ok && deployment.Status.ReadyReplicas == deployment.Status.Replicas {
			deploymentSet[deployment.Name] = true
		}
	}
	for d, found := range deploymentSet {
		if !found {
			return fmt.Errorf("deployment %s is not ready", d)
		}
	}

	return nil
}

func contains(source []string, target string) bool {
	for _, s := range source {
		if s == target {
			return true
		}
	}
	return false
}

// ServerArgsPresent checks if the given arguments are found in the running RKE2 server
func ServerArgsPresent(neededArgs []string) bool {
	currentArgs, err := ServerArgs()
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

// ServerArgs returns the list of arguments that the RKE2 server launched with
func ServerArgs() ([]string, error) {
	results, err := RKE2Cmd("kubectl", "get", "nodes", "-o", `jsonpath='{.items[0].metadata.annotations.rke2\.io/node-args}'`)
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

func k8sClient() (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", "/etc/rancher/rke2/rke2.yaml")
	if err != nil {
		return nil, err
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}
