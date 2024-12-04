package rke2

import (
	"context"
	"sync"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	runtimeClassesChart  = "rke2-runtimeclasses"
	namespace            = "kube-system"
	helm                 = "Helm"
	helmReleaseName      = "meta.helm.sh/release-name"
	helmManageBy         = "app.kubernetes.io/managed-by"
	helmReleaseNamespace = "meta.helm.sh/release-namespace"
)

var runtimes = map[string]bool{
	"nvidia":              true,
	"nvidia-experimental": true,
	"crun":                true,
}

func setRuntimes() cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		go func() {
			defer wg.Done()
			<-args.APIServerReady
			logrus.Info("Setting runtimes")

			client, err := util.GetClientSet(args.KubeConfigSupervisor)
			if err != nil {
				logrus.Fatalf("runtimes: new k8s client: %v", err)
			}

			rcClient := client.NodeV1().RuntimeClasses()

			classes, err := rcClient.List(context.Background(), metav1.ListOptions{})
			if err != nil {
				logrus.Fatalf("runtimes: failed to get runtime classes")
			}

			for _, c := range classes.Items {

				// verify if the runtime class is the runtime class supported by rke2
				if _, ok := runtimes[c.Name]; !ok {
					continue
				}

				if c.Labels == nil {
					c.Labels = map[string]string{}
				}

				if managedBy, ok := c.Labels[helmManageBy]; !ok || managedBy != helm {
					c.Labels[helmManageBy] = helm
				}

				if c.Annotations == nil {
					c.Annotations = map[string]string{}
				}

				if releaseName, ok := c.Annotations[helmReleaseName]; !ok || releaseName != runtimeClassesChart {
					c.Annotations[helmReleaseName] = runtimeClassesChart
				}

				if ns, ok := c.Annotations[helmReleaseNamespace]; !ok || ns != namespace {
					c.Annotations[helmReleaseNamespace] = namespace
				}

				_, err = rcClient.Update(context.Background(), &c, metav1.UpdateOptions{})
				if err != nil {
					logrus.Fatalf("runtimes: failed to update runtime classes")
				}

			}
		}()

		return nil
	}
}
