package staticpod

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/cri"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	pkgerrors "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func binDir(dataDir string) string {
	return filepath.Join(dataDir, "bin")
}

// RemoveDisabledPods deletes the pod manifests for any disabled pods, as well as ensuring that the containers themselves are terminated.
func RemoveDisabledPods(dataDir, containerRuntimeEndpoint string, disabledItems map[string]bool, clusterReset bool) error {
	terminatePods := []string{}
	execPath := binDir(dataDir)
	manifestDir := PodManifestsDir(dataDir)

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
	}

	// check to see if there are manifests for any disabled components. If there are no manifests for
	// disabled components, and termination wasn't forced by cluster-reset, termination is skipped.
	for component, disabled := range disabledItems {
		if disabled {
			manifestName := filepath.Join(manifestDir, component+".yaml")
			if _, err := os.Stat(manifestName); err == nil {
				terminatePods = append(terminatePods, component)
			}
		}
	}

	if len(terminatePods) > 0 {
		logrus.WithField("pods", terminatePods).Infof("Static pod cleanup in progress")
		// delete manifests for disabled items
		for _, component := range terminatePods {
			manifestName := filepath.Join(manifestDir, component+".yaml")
			if err := os.RemoveAll(manifestName); err != nil {
				return pkgerrors.WithMessagef(err, "unable to delete %s manifest", component)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), (5 * time.Minute))
		defer cancel()

		containerdErr := make(chan error)

		// start containerd, if necessary. The command will be terminated automatically when the context is cancelled.
		if containerRuntimeEndpoint == ContainerdSock {
			containerdCmd := exec.CommandContext(ctx, filepath.Join(execPath, "containerd"))
			go startContainerd(ctx, dataDir, containerdErr, containerdCmd)
		}
		// terminate any running containers from the disabled items list
		go terminateRunningContainers(ctx, containerRuntimeEndpoint, terminatePods, containerdErr)

		for {
			select {
			case err := <-containerdErr:
				if err != nil {
					return pkgerrors.WithMessage(err, "temporary containerd process exited unexpectedly")
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

func startContainerd(_ context.Context, dataDir string, errChan chan error, cmd *exec.Cmd) {
	args := []string{
		"-c", filepath.Join(dataDir, "agent", "etc", "containerd", "config.toml"),
		"-a", ContainerdSock,
		"--state", filepath.Dir(ContainerdSock),
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

func terminateRunningContainers(ctx context.Context, containerRuntimeEndpoint string, terminatePods []string, containerdErr chan error) {
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

		for _, component := range terminatePods {
			var found bool
			for _, pod := range resp.Items {
				if pod.Labels["component"] == component && pod.Annotations["kubernetes.io/config.source"] == "file" {
					found = true
					logrus.Infof("Removing pod %s", pod.Metadata.Name)
					if _, err := cRuntime.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: pod.Id}); err != nil {
						logrus.Warnf("Failed to remove pod %s: %v", pod.Id, err)
					}
				}
			}
			// no pods found for this component or not disabled, remove it from the list to be checked
			if !found {
				terminatePods = slices.DeleteFunc(terminatePods, func(c string) bool { return c == component })
			}
		}

		// once all disabled components have been removed, stop polling
		return len(terminatePods) == 0, nil
	})
}
