package rke2

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	toolswatch "k8s.io/client-go/tools/watch"
)

var (
	removalAnnotation = "etcd." + version.Program + ".cattle.io/remove"
)

// cleanupStaticPodsOnSelfDelete returns a StartupHook that will start a
// goroutine to watch for deletion of the local node, and trigger static pod
// cleanup when this occurs.
func cleanupStaticPodsOnSelfDelete(dataDir string) cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		go func() {
			defer wg.Done()
			<-args.APIServerReady
			cs, err := util.GetClientSet(args.KubeConfigSupervisor)
			if err != nil {
				logrus.Fatalf("spc: new k8s client: %v", err)
			}
			go watchForSelfDelete(ctx, dataDir, cs)
		}()
		return nil
	}
}

// watchForSelfDelete watches for delete of the local node. When a delete event
// is found, it calls static pod cleanup.  Much of this is cribbed from
// kubernetes/cmd/kube-proxy/app/server_others.go
func watchForSelfDelete(ctx context.Context, dataDir string, client kubernetes.Interface) {
	nodeName := os.Getenv("NODE_NAME")
	logrus.Infof("Watching for delete of %s Node object", nodeName)
	fieldSelector := fields.Set{metav1.ObjectNameField: nodeName}.String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (object runtime.Object, e error) {
			options.FieldSelector = fieldSelector
			return client.CoreV1().Nodes().List(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return client.CoreV1().Nodes().Watch(ctx, options)
		},
	}
	condition := func(event watch.Event) (bool, error) {
		if n, ok := event.Object.(*v1.Node); ok {
			if n.ObjectMeta.DeletionTimestamp != nil || n.Annotations[removalAnnotation] == "true" {
				return true, nil
			}
			return false, nil
		}
		return false, fmt.Errorf("event object not of type Node")
	}

	_, err := toolswatch.UntilWithSync(ctx, lw, &v1.Node{}, nil, condition)
	if err != nil && !errors.Is(err, context.Canceled) {
		logrus.Errorf("spc: failed to wait for node deletion: %v", err)
		return
	}

	logrus.Infof("Local Node deleted or removed from etcd cluster, cleaning up server static pods")
	if err := cleanupStaticPods(dataDir); err != nil {
		logrus.Errorf("spc: failed to clean up static pods: %v", err)
	}
}

// cleanupStaticPods deletes all the control-plane and etc static pod manifests.
func cleanupStaticPods(dataDir string) error {
	components := []string{"kube-apiserver", "kube-scheduler", "kube-controller-manager", "cloud-controller-manager", "etcd"}
	manifestDir := podManifestsDir(dataDir)
	for _, component := range components {
		manifestName := filepath.Join(manifestDir, component+".yaml")
		if err := os.RemoveAll(manifestName); err != nil {
			return errors.Wrapf(err, "unable to delete %s manifest", component)
		}
	}
	return nil
}
