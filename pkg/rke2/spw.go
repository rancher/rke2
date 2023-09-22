package rke2

// TODO: move this into the podexecutor package, this logic is specific to that executor and should be there instead of here.

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/cri"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	crierror "k8s.io/cri-api/pkg/errors"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	runtimeutil "k8s.io/kubernetes/pkg/kubelet/kuberuntime/util"
	netutils "k8s.io/utils/net"
)

type containerInfo struct {
	Config *runtimeapi.ContainerConfig `json:"config,omitempty"`
}

// reconcileStaticPods validates that the running pods for etcd and kube-apiserver match the static pod
// manifests provided in /var/lib/rancher/rke2/agent/pod-manifests. If any old pods are found, they are
// manually terminated, as the kubelet cannot be relied upon to terminate old pod when the apiserver is
// not available.
func reconcileStaticPods(containerRuntimeEndpoint, dataDir string) cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		go func() {
			defer wg.Done()
			if err := wait.PollImmediateWithContext(ctx, 20*time.Second, 30*time.Minute, func(ctx context.Context) (bool, error) {
				if containerRuntimeEndpoint == "" {
					containerRuntimeEndpoint = containerdSock
				}
				conn, err := cri.Connection(ctx, containerRuntimeEndpoint)
				if err != nil {
					logrus.Infof("Waiting for cri connection: %v", err)
					return false, nil
				}
				cRuntime := runtimeapi.NewRuntimeServiceClient(conn)
				defer conn.Close()

				manifestDir := podManifestsDir(dataDir)

				for _, pod := range []string{"etcd", "kube-apiserver"} {
					manifestFile := filepath.Join(manifestDir, pod+".yaml")
					if err := checkManifestDeployed(ctx, cRuntime, manifestFile); err != nil {
						if errors.Is(err, os.ErrNotExist) {
							// Since split-role servers exist, we don't care if no manifest is found
							continue
						}
						logrus.Infof("Pod for %s not synced (%v), retrying", pod, err)
						return false, nil
					}
					logrus.Infof("Pod for %s is synced", pod)
				}
				return true, nil
			}); err != nil {
				logrus.Fatalf("Failed waiting for static pods to sync: %v", err)
			}
		}()
		return nil
	}
}

// checkManifestDeployed verifies that a single pod for this manifest is running with the current pod uid.
// Pod sandboxes with a different uid are removed and an error returned indicating that cleanup is in progress.
func checkManifestDeployed(ctx context.Context, cRuntime runtimeapi.RuntimeServiceClient, manifestFile string) error {
	f, err := os.Open(manifestFile)
	if err != nil {
		return errors.Wrap(err, "failed to open manifest")
	}
	defer f.Close()

	pod := &v1.Pod{}
	decoder := yaml.NewYAMLToJSONDecoder(f)
	err = decoder.Decode(pod)
	if err != nil {
		return errors.Wrap(err, "failed to decode manifest")
	}

	filter := &runtimeapi.PodSandboxFilter{
		LabelSelector: map[string]string{
			"component":                   pod.Labels["component"],
			"io.kubernetes.pod.namespace": pod.Namespace,
			"tier":                        pod.Labels["tier"],
		},
	}
	resp, err := cRuntime.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{Filter: filter})
	if err != nil {
		return errors.Wrap(err, "failed to list pod sandboxes")
	}

	podStatus := &kubecontainer.PodStatus{
		ID:              pod.UID,
		Name:            pod.Name,
		Namespace:       pod.Namespace,
		SandboxStatuses: []*runtimeapi.PodSandboxStatus{},
	}

	// Get detailed pod sandbox status for any sandboxes associated with the current pod,
	// so that we can use kubelet runtime logic to determine which is the latest.
	// Ref: https://github.com/kubernetes/kubernetes/blob/v1.28.2/pkg/kubelet/kuberuntime/kuberuntime_manager.go#L1404
	matchingPodIdx := 0
	for _, podSandbox := range resp.Items {
		if podSandbox.Labels["io.kubernetes.pod.uid"] != string(pod.UID) {
			continue
		}
		statusResp, err := cRuntime.PodSandboxStatus(ctx, &runtimeapi.PodSandboxStatusRequest{PodSandboxId: podSandbox.Id})
		if crierror.IsNotFound(err) {
			continue
		}
		if err != nil {
			return errors.Wrap(err, "failed to get pod sandbox status")
		}
		podStatus.SandboxStatuses = append(podStatus.SandboxStatuses, statusResp.Status)
		// only get pod IP from the latest sandbox
		if matchingPodIdx == 0 && statusResp.Status.State == runtimeapi.PodSandboxState_SANDBOX_READY {
			podStatus.IPs = determinePodSandboxIPs(statusResp.Status)
		}
		matchingPodIdx++
	}

	// Use kubelet runtime logic to find the latest pod sandbox
	newSandboxNeeded, _, sandboxID := runtimeutil.PodSandboxChanged(pod, podStatus)

	// Remove any pod sandboxes that are not the latest
	var sandboxRemoved bool
	for _, podSandbox := range resp.Items {
		if podSandbox.Labels["io.kubernetes.pod.uid"] != string(pod.UID) || (sandboxID != "" && sandboxID != podSandbox.Id) {
			sandboxRemoved = true
			if _, err := cRuntime.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: podSandbox.Id}); err != nil {
				logrus.Warnf("Failed to remove old %s pod sandbox: %v", pod.Name, err)
			}
		}
	}

	if sandboxRemoved {
		return errors.New("waiting for termination of old pod sandbox")
	}
	if newSandboxNeeded {
		if sandboxID != "" {
			return errors.New("pod sandbox has changed")
		}
		return errors.New("pod sandbox not found")
	}
	return nil
}

// determinePodSandboxIP determines the IP addresses of the given pod sandbox.
// The list may be empty if the pod uses host networking.
// Ref: https://github.com/kubernetes/kubernetes/blob/v1.28.2/pkg/kubelet/kuberuntime/kuberuntime_sandbox.go#L305
func determinePodSandboxIPs(podSandbox *runtimeapi.PodSandboxStatus) []string {
	podIPs := make([]string, 0)
	if podSandbox.Network == nil {
		return podIPs
	}

	// pick primary IP
	if len(podSandbox.Network.Ip) != 0 {
		if netutils.ParseIPSloppy(podSandbox.Network.Ip) == nil {
			return nil
		}
		podIPs = append(podIPs, podSandbox.Network.Ip)
	}

	// pick additional ips, if cri reported them
	for _, podIP := range podSandbox.Network.AdditionalIps {
		if nil == netutils.ParseIPSloppy(podIP.Ip) {
			return nil
		}
		podIPs = append(podIPs, podIP.Ip)
	}

	return podIPs
}
