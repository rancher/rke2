package rke2

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	containerdk3s "github.com/k3s-io/k3s/pkg/agent/containerd"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// total of 4 minutes
var podCheckBackoff = wait.Backoff{
	Steps:    12,
	Duration: 15 * time.Second,
	Factor:   1.0,
	Jitter:   0.1,
}

// total of 31 seconds
var criBackoff = wait.Backoff{
	Steps:    5,
	Duration: 1 * time.Second,
	Factor:   2,
	Jitter:   0.1,
}

// checkStaticManifests validates that the pods started with rke2 match the static manifests
// provided in /var/lib/rancher/rke2/agent/pod-manifests. When restarting rke2, it takes time
// for any changes to static manifests to be pulled by kubelet. Additionally this prevents errors
// where something is wrong with the static manifests and RKE2 starts anyways.
func checkStaticManifests(dataDir string) cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		go func() {
			defer wg.Done()
			<-args.APIServerReady

			var conn *grpc.ClientConn
			if err := wait.ExponentialBackoff(criBackoff, func() (done bool, err error) {
				conn, err = containerdk3s.CriConnection(ctx, containerdSock)
				if err != nil {
					logrus.Infof("Waiting for cri connection: %v", err)
					return false, nil
				}
				return true, nil
			}); err != nil {
				logrus.Fatalf("failed to setup cri connection: %v", err)
			}
			cRuntime := runtimeapi.NewRuntimeServiceClient(conn)
			defer conn.Close()

			manifestDir := podManifestsDir(dataDir)
			etcdManifest := filepath.Join(manifestDir, "etcd.yaml")
			kubeAPIManifest := filepath.Join(manifestDir, "kube-apiserver.yaml")

			// Extract UID from manifest and compare it to the running pod
			if f, err := os.Open(etcdManifest); err == nil {
				if err != nil {
					logrus.Fatal(err)
				}
				podManifest := v1.Pod{}
				decoder := yaml.NewYAMLToJSONDecoder(f)
				err = decoder.Decode(&podManifest)
				if err != nil {
					logrus.Fatal(err)
				}
				etcdFilter := &runtimeapi.ContainerFilter{
					LabelSelector: map[string]string{
						"io.kubernetes.container.name": "etcd",
					},
				}
				if err := wait.ExponentialBackoff(podCheckBackoff, func() (done bool, err error) {
					resp, err := cRuntime.ListContainers(ctx, &runtimeapi.ListContainersRequest{Filter: etcdFilter})
					if err != nil {
						return false, err
					}
					for _, c := range resp.Containers {
						if c.Labels["io.kubernetes.pod.uid"] == string(podManifest.UID) {
							logrus.Info("Latest etcd manifest deployed")
							return true, nil
						}
					}
					logrus.Info("Waiting for etcd manifest")
					return false, nil
				}); err != nil {
					logrus.Fatal(err)
				}

			} else if !errors.Is(err, os.ErrNotExist) {
				// Since split-role servers exist, we don't care if no etcd manifest is found
				logrus.Fatal(err)
			}

			if f, err := os.Open(kubeAPIManifest); err == nil {
				podManifest := v1.Pod{}
				decoder := yaml.NewYAMLToJSONDecoder(f)
				err = decoder.Decode(&podManifest)
				if err != nil {
					logrus.Fatal(err)
				}

				kubeAPIFilter := &runtimeapi.ContainerFilter{
					LabelSelector: map[string]string{
						"io.kubernetes.container.name": "kube-apiserver",
					},
				}

				if err := wait.ExponentialBackoff(podCheckBackoff, func() (done bool, err error) {
					resp, err := cRuntime.ListContainers(ctx, &runtimeapi.ListContainersRequest{Filter: kubeAPIFilter})
					if err != nil {
						return false, err
					}
					for _, c := range resp.Containers {
						if c.Labels["io.kubernetes.pod.uid"] == string(podManifest.UID) {
							logrus.Info("Latest kube-apiserver manifest deployed")
							return true, nil
						}
					}
					logrus.Info("Waiting for kube-apiserver manifest")
					return false, nil
				}); err != nil {
					logrus.Fatal(err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				// Since split-role servers exist, we don't care if no kube-apiserver manifest is found
				logrus.Fatal(err)
			}
		}()
		return nil
	}
}
