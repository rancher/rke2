package rke2

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent/config"
	containerdk3s "github.com/rancher/k3s/pkg/agent/containerd"
	"github.com/rancher/k3s/pkg/cli/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cli/etcdsnapshot"
	"github.com/rancher/k3s/pkg/cli/server"
	daemonconfig "github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/executor"
	rawServer "github.com/rancher/k3s/pkg/server"
	"github.com/rancher/rke2/pkg/controllers/cisnetworkpolicy"
	"github.com/rancher/rke2/pkg/images"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

type Config struct {
	AuditPolicyFile              string
	CloudProviderConfig          string
	CloudProviderName            string
	Images                       images.ImageOverrideConfig
	KubeletPath                  string
	ControlPlaneResourceRequests string
	ControlPlaneResourceLimits   string
	ExtraMounts                  ExtraMounts
	ExtraEnv                     ExtraEnv
}

type ExtraMounts struct {
	KubeAPIServer          cli.StringSlice
	KubeScheduler          cli.StringSlice
	KubeControllerManager  cli.StringSlice
	KubeProxy              cli.StringSlice
	Etcd                   cli.StringSlice
	CloudControllerManager cli.StringSlice
}

type ExtraEnv struct {
	KubeAPIServer          cli.StringSlice
	KubeScheduler          cli.StringSlice
	KubeControllerManager  cli.StringSlice
	KubeProxy              cli.StringSlice
	Etcd                   cli.StringSlice
	CloudControllerManager cli.StringSlice
}

// Valid CIS Profile versions
const (
	CISProfile15           = "cis-1.5"
	CISProfile16           = "cis-1.6"
	defaultAuditPolicyFile = "/etc/rancher/rke2/audit-policy.yaml"
	containerdSock         = "/run/k3s/containerd/containerd.sock"
	KubeAPIServer          = "kube-apiserver"
	KubeScheduler          = "kube-scheduler"
	KubeControllerManager  = "kube-controller-manager"
	KubeProxy              = "kube-proxy"
	Etcd                   = "etcd"
	CloudControllerManager = "cloud-controller-manager"
)

func Server(clx *cli.Context, cfg Config) error {
	if err := setup(clx, cfg, true); err != nil {
		return err
	}

	if err := clx.Set("secrets-encryption", "true"); err != nil {
		return err
	}

	// Disable all disableable k3s packaged components. In addition to manifests,
	// this also disables several integrated controllers.
	disableItems := strings.Split(cmds.DisableItems, ",")
	for _, item := range disableItems {
		if err := clx.Set("disable", strings.TrimSpace(item)); err != nil {
			return err
		}
	}
	cisMode := isCISMode(clx)
	defaultNamespaces := []string{
		metav1.NamespaceSystem,
		metav1.NamespaceDefault,
		metav1.NamespacePublic,
	}
	dataDir := clx.String("data-dir")
	cmds.ServerConfig.StartupHooks = append(cmds.ServerConfig.StartupHooks,
		setPSPs(cisMode),
		setNetworkPolicies(cisMode, defaultNamespaces),
		setClusterRoles(),
		restrictServiceAccounts(cisMode, defaultNamespaces),
		setKubeProxyDisabled(),
		cleanupStaticPodsOnSelfDelete(dataDir),
	)

	var leaderControllers rawServer.CustomControllers

	if cisMode {
		leaderControllers = append(leaderControllers, cisnetworkpolicy.Controller)
	}

	return server.RunWithControllers(clx, leaderControllers, rawServer.CustomControllers{})
}

func Agent(clx *cli.Context, cfg Config) error {
	if err := setup(clx, cfg, false); err != nil {
		return err
	}
	return agent.Run(clx)
}

func EtcdSnapshot(clx *cli.Context, cfg Config) error {
	cmds.ServerConfig.DatastoreEndpoint = "etcd"
	return etcdsnapshot.Run(clx)
}

func setup(clx *cli.Context, cfg Config, isServer bool) error {
	dataDir := clx.String("data-dir")
	disableETCD := clx.Bool("disable-etcd")
	disableScheduler := clx.Bool("disable-scheduler")
	disableAPIServer := clx.Bool("disable-apiserver")
	disableControllerManager := clx.Bool("disable-controller-manager")
	disableCloudControllerManager := clx.Bool("disable-cloud-controller")
	clusterReset := clx.Bool("cluster-reset")

	ex, err := initExecutor(clx, cfg, dataDir, disableETCD, isServer)
	if err != nil {
		return err
	}
	executor.Set(ex)

	// check for force restart file
	var forceRestart bool
	if _, err := os.Stat(ForceRestartFile(dataDir)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		forceRestart = true
		os.Remove(ForceRestartFile(dataDir))
	}
	disabledItems := map[string]bool{
		"kube-apiserver":           disableAPIServer || forceRestart,
		"kube-scheduler":           disableScheduler || forceRestart,
		"kube-controller-manager":  disableControllerManager || forceRestart,
		"cloud-controller-manager": disableCloudControllerManager || forceRestart,
		"etcd":                     disableETCD || forceRestart,
	}
	return removeOldPodManifests(dataDir, disabledItems, clusterReset)
}

func ForceRestartFile(dataDir string) string {
	return filepath.Join(dataDir, "force-restart")
}

func podManifestsDir(dataDir string) string {
	return filepath.Join(dataDir, "agent", config.DefaultPodManifestPath)
}

func binDir(dataDir string) string {
	return filepath.Join(dataDir, "bin")
}

func removeOldPodManifests(dataDir string, disabledItems map[string]bool, clusterReset bool) error {
	kubeletStandAlone := false
	execPath := binDir(dataDir)
	manifestDir := podManifestsDir(dataDir)

	// no need to clean up static pods if this is a clean install (bin or manifests dirs missing)
	for _, path := range []string{execPath, manifestDir} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil
		}
	}

	// ensure etcd manifest is removed if cluster-reset is passed, and force
	// standalone startup to ensure static pods are terminated
	if clusterReset {
		disabledItems["etcd"] = true
		kubeletStandAlone = true
	}

	// check to see if there are manifests for any disabled components
	for component, disabled := range disabledItems {
		if disabled {
			manifestName := filepath.Join(manifestDir, component+".yaml")
			if _, err := os.Stat(manifestName); err == nil {
				kubeletStandAlone = true
			}
		}
	}

	if kubeletStandAlone {
		// delete all manifests
		for component := range disabledItems {
			manifestName := filepath.Join(manifestDir, component+".yaml")
			if err := os.RemoveAll(manifestName); err != nil {
				return errors.Wrapf(err, "unable to delete %s manifest", component)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), (5 * time.Minute))
		defer cancel()

		kubeletCmd := exec.CommandContext(ctx, filepath.Join(execPath, "kubelet"))
		containerdCmd := exec.CommandContext(ctx, filepath.Join(execPath, "containerd"))

		kubeletErr := make(chan error)
		containerdErr := make(chan error)

		// start containerd
		go startContainerd(dataDir, containerdErr, containerdCmd)
		// start kubelet
		go startKubelet(dataDir, kubeletErr, kubeletCmd)
		// check for any running containers from the disabled items list
		go checkForRunningContainers(ctx, disabledItems, kubeletErr, containerdErr)

		// ensure temporary kubelet and containerd are terminated
		defer func() {
			if kubeletCmd.Process != nil {
				kubeletCmd.Process.Kill()
			}
			if containerdCmd.Process != nil {
				containerdCmd.Process.Kill()
			}
		}()

		for {
			select {
			case err := <-kubeletErr:
				if err != nil {
					return errors.Wrap(err, "temporary kubelet process exited unexpectedly")
				}
			case err := <-containerdErr:
				if err != nil {
					return errors.Wrap(err, "temporary containerd process exited unexpectedly")
				}
			case <-ctx.Done():
				return errors.New("static pod cleanup timed out")
			}
			logrus.Info("Static pod cleanup completed successfully")
			break
		}
	}

	return nil
}

func isCISMode(clx *cli.Context) bool {
	profile := clx.String("profile")
	return profile == CISProfile15 || profile == CISProfile16
}

func startKubelet(dataDir string, errChan chan error, cmd *exec.Cmd) {
	args := []string{
		"--fail-swap-on=false",
		"--container-runtime=remote",
		"--containerd=" + containerdSock,
		"--container-runtime-endpoint=unix://" + containerdSock,
		"--pod-manifest-path=" + podManifestsDir(dataDir),
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PATH=%s:%s", binDir(dataDir), os.Getenv("PATH")))
	cmd.Env = append(cmd.Env, "NOTIFY_SOCKET=")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logrus.Infof("Running temporary kubelet %s", daemonconfig.ArgString(cmd.Args))
	errChan <- cmd.Run()
}

func startContainerd(dataDir string, errChan chan error, cmd *exec.Cmd) {
	args := []string{
		"-c", filepath.Join(dataDir, "agent", "etc", "containerd", "config.toml"),
		"-a", containerdSock,
		"--state", filepath.Dir(containerdSock),
		"--root", filepath.Join(dataDir, "agent", "containerd"),
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PATH=%s:%s", binDir(dataDir), os.Getenv("PATH")))
	cmd.Env = append(cmd.Env, "NOTIFY_SOCKET=")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logrus.Infof("Running temporary containerd %s", daemonconfig.ArgString(cmd.Args))
	errChan <- cmd.Run()
}

func isContainerRunning(name string, resp *runtimeapi.ListContainersResponse) bool {
	for _, c := range resp.Containers {
		if c.Labels["io.kubernetes.pod.namespace"] == metav1.NamespaceSystem &&
			strings.HasPrefix(c.Labels["io.kubernetes.pod.name"], name) &&
			c.Labels["io.kubernetes.container.name"] == name {
			return true
		}
	}
	return false
}

func checkForRunningContainers(ctx context.Context, disabledItems map[string]bool, kubeletErr, containerdErr chan error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		conn, err := containerdk3s.CriConnection(ctx, containerdSock)
		if err != nil {
			logrus.Warnf("Failed to setup cri connection: %v", err)
			continue
		}
		c := runtimeapi.NewRuntimeServiceClient(conn)
		defer conn.Close()
		resp, err := c.ListContainers(ctx, &runtimeapi.ListContainersRequest{})
		if err != nil {
			logrus.Warnf("Failed to list containers: %v", err)
			continue
		}
		containersRunning := false
		for item := range disabledItems {
			if isContainerRunning(item, resp) {
				logrus.Infof("Waiting for deletion of %s static pod", item)
				containersRunning = true
				break
			}
		}
		if containersRunning {
			continue
		}
		// if all disabled item containers have been removed,
		// send on the subprocess error channels to wake up the select
		// loop and shut everything down.
		containerdErr <- nil
		kubeletErr <- nil
		break
	}
}
