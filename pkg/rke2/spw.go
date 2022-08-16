package rke2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	containerdk3s "github.com/k3s-io/k3s/pkg/agent/containerd"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// checkStaticManifests validates that the pods started with rke2 match the static manifests
// provided in /var/lib/rancher/rke2/agent/pod-manifests. When restarting rke2, it takes time
// for any changes to static manifests to be pulled by kubelet. Additionally this prevents errors
// where something is wrong with the static manifests and RKE2 starts anyways.
func checkStaticManifests(containerRuntimeEndpoint, dataDir string) cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		go func() {
			defer wg.Done()
			if err := wait.PollImmediate(20*time.Second, 30*time.Minute, func() (bool, error) {
				if containerRuntimeEndpoint == "" {
					containerRuntimeEndpoint = containerdSock
				}
				conn, err := containerdk3s.CriConnection(ctx, containerRuntimeEndpoint)
				if err != nil {
					logrus.Infof("Waiting for cri connection: %v", err)
					return false, nil
				}
				cRuntime := runtimeapi.NewRuntimeServiceClient(conn)
				defer conn.Close()

				manifestDir := podManifestsDir(dataDir)

				for _, pod := range []string{"etcd", "kube-apiserver"} {
					manifestFile := filepath.Join(manifestDir, pod+".yaml")
					if f, err := os.Open(manifestFile); err == nil {
						defer f.Close()
						podManifest := v1.Pod{}
						decoder := yaml.NewYAMLToJSONDecoder(f)
						err = decoder.Decode(&podManifest)
						if err != nil {
							logrus.Fatalf("Failed to decode %s manifest: %v", pod, err)
						}
						podFilter := &runtimeapi.ContainerFilter{
							LabelSelector: map[string]string{
								"io.kubernetes.pod.uid": string(podManifest.UID),
							},
						}
						resp, err := cRuntime.ListContainers(ctx, &runtimeapi.ListContainersRequest{Filter: podFilter})
						if err != nil {
							return false, err
						}
						if len(resp.Containers) < 1 {
							logrus.Infof("%s pod not found, retrying", pod)
							return false, nil
						}
						logrus.Infof("Latest %s manifest deployed", pod)
					} else if !errors.Is(err, os.ErrNotExist) {
						// Since split-role servers exist, we don't care if no manifest is found
						return false, fmt.Errorf("failed to open %s manifest: %v", pod, err)
					}
				}
				return true, nil
			}); err != nil {
				logrus.Fatalf("Failed waiting for manifests to deploy: %v", err)
			}
		}()
		return nil
	}
}
