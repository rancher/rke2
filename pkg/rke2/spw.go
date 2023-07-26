package rke2

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
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
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

// checkManifestDeployed returns an error if the static pod's manifest cannot be decoded and verified as present
// and exclusively running with the current pod uid. If old pods are found, they will be terminated and an error returned.
func checkManifestDeployed(ctx context.Context, cRuntime runtimeapi.RuntimeServiceClient, manifestFile string) error {
	f, err := os.Open(manifestFile)
	if err != nil {
		return errors.Wrap(err, "failed to open manifest")
	}
	defer f.Close()

	podManifest := v1.Pod{}
	decoder := yaml.NewYAMLToJSONDecoder(f)
	err = decoder.Decode(&podManifest)
	if err != nil {
		return errors.Wrap(err, "failed to decode manifest")
	}

	filter := &runtimeapi.PodSandboxFilter{
		LabelSelector: map[string]string{
			"component":                   podManifest.Labels["component"],
			"io.kubernetes.pod.namespace": podManifest.Namespace,
			"tier":                        podManifest.Labels["tier"],
		},
	}
	resp, err := cRuntime.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{Filter: filter})
	if err != nil {
		return errors.Wrap(err, "failed to list pods")
	}

	var currentPod, stalePod bool
	for _, pod := range resp.Items {
		if pod.Annotations["kubernetes.io/config.source"] != "file" {
			continue
		}
		if pod.Labels["io.kubernetes.pod.uid"] == string(podManifest.UID) {
			currentPod = pod.State == runtimeapi.PodSandboxState_SANDBOX_READY
		} else {
			stalePod = true
			if _, err := cRuntime.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: pod.Id}); err != nil {
				logrus.Warnf("Failed to terminate old %s pod: %v", pod.Metadata.Name, err)
			}
		}
	}

	if stalePod {
		return errors.New("waiting for termination of old pod")
	}
	if !currentPod {
		return errors.New("no current running pod found")
	}
	return nil
}
