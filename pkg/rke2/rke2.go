package rke2

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/config"
	"github.com/k3s-io/k3s/pkg/agent/cri"
	"github.com/k3s-io/k3s/pkg/cli/agent"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/server"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	rawServer "github.com/k3s-io/k3s/pkg/server"
	"github.com/natefinch/lumberjack"
	"github.com/pkg/errors"
	"github.com/rancher/rke2/pkg/controllers/cisnetworkpolicy"
	"github.com/rancher/rke2/pkg/images"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type Config struct {
	AuditPolicyFile                string
	PodSecurityAdmissionConfigFile string
	CloudProviderConfig            string
	CloudProviderName              string
	Images                         images.ImageOverrideConfig
	KubeletPath                    string
	ControlPlaneResourceRequests   cli.StringSlice
	ControlPlaneResourceLimits     cli.StringSlice
	ControlPlaneProbeConf          cli.StringSlice
	ExtraMounts                    ExtraMounts
	ExtraEnv                       ExtraEnv
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
	CISProfile123          = "cis-1.23"
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
		item = strings.TrimSpace(item)
		if clx.Bool("enable-" + item) {
			continue
		}
		if err := clx.Set("disable", item); err != nil {
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
		checkStaticManifests(cmds.AgentConfig.ContainerRuntimeEndpoint, dataDir),
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

func setup(clx *cli.Context, cfg Config, isServer bool) error {
	dataDir := clx.String("data-dir")
	clusterReset := clx.Bool("cluster-reset")
	clusterResetRestorePath := clx.String("cluster-reset-restore-path")
	containerRuntimeEndpoint := clx.String("container-runtime-endpoint")

	ex, err := initExecutor(clx, cfg, isServer)
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
		"cloud-controller-manager": !isServer || forceRestart || clx.Bool("disable-cloud-controller"),
		"etcd":                     !isServer || forceRestart || clx.Bool("disable-etcd"),
		"kube-apiserver":           !isServer || forceRestart || clx.Bool("disable-apiserver"),
		"kube-controller-manager":  !isServer || forceRestart || clx.Bool("disable-controller-manager"),
		"kube-scheduler":           !isServer || forceRestart || clx.Bool("disable-scheduler"),
	}
	// adding force restart file when cluster reset restore path is passed
	if clusterResetRestorePath != "" {
		forceRestartFile := ForceRestartFile(dataDir)
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return err
		}
		if err := ioutil.WriteFile(forceRestartFile, []byte(""), 0600); err != nil {
			return err
		}
	}
	return removeDisabledPods(dataDir, containerRuntimeEndpoint, disabledItems, clusterReset)
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

// removeDisabledPods deletes the pod manifests for any disabled pods, as well as ensuring that the containers themselves are terminated.
func removeDisabledPods(dataDir, containerRuntimeEndpoint string, disabledItems map[string]bool, clusterReset bool) error {
	terminatePods := false
	execPath := binDir(dataDir)
	manifestDir := podManifestsDir(dataDir)

	// no need to clean up static pods if this is a clean install (bin or manifests dirs missing)
	for _, path := range []string{execPath, manifestDir} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil
		}
	}

	// ensure etcd and the apiserver are terminated if doing a cluster-reset, and force pod
	// termination even if there are no manifests on disk
	if clusterReset {
		disabledItems["etcd"] = true
		disabledItems["kube-apiserver"] = true
		terminatePods = true
	}

	// check to see if there are manifests for any disabled components. If there are no manifests for
	// disabled components, and termination wasn't forced by cluster-reset, termination is skipped.
	for component, disabled := range disabledItems {
		if disabled {
			manifestName := filepath.Join(manifestDir, component+".yaml")
			if _, err := os.Stat(manifestName); err == nil {
				terminatePods = true
			}
		}
	}

	if terminatePods {
		logrus.Infof("Static pod cleanup in progress")
		// delete manifests for disabled items
		for component, disabled := range disabledItems {
			if disabled {
				manifestName := filepath.Join(manifestDir, component+".yaml")
				if err := os.RemoveAll(manifestName); err != nil {
					return errors.Wrapf(err, "unable to delete %s manifest", component)
				}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), (5 * time.Minute))
		defer cancel()

		containerdErr := make(chan error)

		// start containerd, if necessary. The command will be terminated automatically when the context is cancelled.
		if containerRuntimeEndpoint == "" {
			containerdCmd := exec.CommandContext(ctx, filepath.Join(execPath, "containerd"))
			go startContainerd(ctx, dataDir, containerdErr, containerdCmd)
		}
		// terminate any running containers from the disabled items list
		go terminateRunningContainers(ctx, containerRuntimeEndpoint, disabledItems, containerdErr)

		for {
			select {
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
	return profile == CISProfile123
}

func startContainerd(_ context.Context, dataDir string, errChan chan error, cmd *exec.Cmd) {
	args := []string{
		"-c", filepath.Join(dataDir, "agent", "etc", "containerd", "config.toml"),
		"-a", containerdSock,
		"--state", filepath.Dir(containerdSock),
		"--root", filepath.Join(dataDir, "agent", "containerd"),
	}

	logFile := filepath.Join(dataDir, "agent", "containerd", "containerd.log")
	logrus.Infof("Logging temporary containerd to %s", logFile)
	logOut := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    50,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
	}

	env := []string{}
	cenv := []string{}

	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		switch {
		case pair[0] == "NOTIFY_SOCKET":
			// elide NOTIFY_SOCKET to prevent spurious notifications to systemd
		case pair[0] == "CONTAINERD_LOG_LEVEL":
			// Turn CONTAINERD_LOG_LEVEL variable into log-level flag
			args = append(args, "--log-level", pair[1])
		case strings.HasPrefix(pair[0], "CONTAINERD_"):
			// Strip variables with CONTAINERD_ prefix before passing through
			// This allows doing things like setting a proxy for image pulls by setting
			// CONTAINERD_https_proxy=http://proxy.example.com:8080
			pair[0] = strings.TrimPrefix(pair[0], "CONTAINERD_")
			cenv = append(cenv, strings.Join(pair, "="))
		case pair[0] == "PATH":
			env = append(env, fmt.Sprintf("PATH=%s:%s", binDir(dataDir), pair[1]))
		default:
			env = append(env, strings.Join(pair, "="))
		}
	}

	cmd.Args = append(cmd.Args, args...)
	cmd.Env = append(env, cenv...)
	cmd.Stdout = logOut
	cmd.Stderr = logOut

	logrus.Infof("Running temporary containerd %s", daemonconfig.ArgString(cmd.Args))
	errChan <- cmd.Run()
}

func terminateRunningContainers(ctx context.Context, containerRuntimeEndpoint string, disabledItems map[string]bool, containerdErr chan error) {
	if containerRuntimeEndpoint == "" {
		containerRuntimeEndpoint = containerdSock
	}

	// send on the subprocess error channel to wake up the select
	// loop and shut everything down when the poll completes
	containerdErr <- wait.PollUntilWithContext(ctx, 10*time.Second, func(ctx context.Context) (bool, error) {
		conn, err := cri.Connection(ctx, containerRuntimeEndpoint)
		if err != nil {
			logrus.Warnf("Failed to open CRI connection: %v", err)
			return false, nil
		}
		defer conn.Close()

		// List all pods in the kube-system namespace; it's faster than asking for them one by
		// one since we're going to be iterating over a list of components.
		cRuntime := runtimeapi.NewRuntimeServiceClient(conn)
		filter := &runtimeapi.PodSandboxFilter{LabelSelector: map[string]string{"io.kubernetes.pod.namespace": metav1.NamespaceSystem}}
		resp, err := cRuntime.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{Filter: filter})
		if err != nil {
			logrus.Warnf("Failed to list pods: %v", err)
			return false, nil
		}

		for component, disabled := range disabledItems {
			var found bool
			for _, pod := range resp.Items {
				if disabled && pod.Labels["component"] == component && pod.Annotations["kubernetes.io/config.source"] == "file" {
					found = true
					logrus.Infof("Removing pod %s", pod.Metadata.Name)
					if _, err := cRuntime.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: pod.Id}); err != nil {
						logrus.Warnf("Failed to remove pod %s: %v", pod.Id, err)
					}
				}
			}
			// no pods found for this component or not disabled, remove it from the list to be checked
			if !found || !disabled {
				delete(disabledItems, component)
			}
		}

		// once all disabled components have been removed, stop polling
		return len(disabledItems) == 0, nil
	})
}
