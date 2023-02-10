package rke2

import (
	"context"
	"encoding/json"
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
)

type containerInfo struct {
	Config *runtimeapi.ContainerConfig `json:"config,omitempty"`
}

// checkStaticManifests validates that the pods started with rke2 match the static manifests
// provided in /var/lib/rancher/rke2/agent/pod-manifests. When restarting rke2, it takes time
// for any changes to static manifests to be pulled by kubelet. Additionally this prevents errors
// where something is wrong with the static manifests and RKE2 starts anyways.
func checkStaticManifests(containerRuntimeEndpoint, dataDir string) cmds.StartupHook {
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
						logrus.Infof("Container for %s not found (%v), retrying", pod, err)
						return false, nil
					}
					logrus.Infof("Container for %s is running", pod)
				}
				return true, nil
			}); err != nil {
				logrus.Fatalf("Failed waiting for static pods to deploy: %v", err)
			}
		}()
		return nil
	}
}

// checkManifestDeployed returns an error if the static pod's manifest cannot be decoded and
// verified as present and running with the current pod hash in the container runtime.
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

	var podHash string
	for _, env := range podManifest.Spec.Containers[0].Env {
		if env.Name == "POD_HASH" {
			podHash = env.Value
			break
		}
	}

	filter := &runtimeapi.ContainerFilter{
		State: &runtimeapi.ContainerStateValue{
			State: runtimeapi.ContainerState_CONTAINER_RUNNING,
		},
		LabelSelector: map[string]string{
			"io.kubernetes.pod.uid": string(podManifest.UID),
		},
	}

	resp, err := cRuntime.ListContainers(ctx, &runtimeapi.ListContainersRequest{Filter: filter})
	if err != nil {
		return errors.Wrap(err, "failed to list containers")
	}

	for _, container := range resp.Containers {
		resp, err := cRuntime.ContainerStatus(ctx, &runtimeapi.ContainerStatusRequest{ContainerId: container.Id, Verbose: true})
		if err != nil {
			return errors.Wrap(err, "failed to get container status")
		}
		info := &containerInfo{}
		err = json.Unmarshal([]byte(resp.Info["info"]), &info)
		if err != nil || info.Config == nil {
			return errors.Wrap(err, "failed to unmarshal container config")
		}
		for _, env := range info.Config.Envs {
			if env.Key == "POD_HASH" && env.Value == podHash {
				return nil
			}
		}
	}
	return errors.New("no matching container found")
}
