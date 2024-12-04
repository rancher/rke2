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
	runtimeClassesChart = "rke2-runtimeclasses"

	// Values from upstream, see reference at -> https://github.com/helm/helm/blob/v3.16.3/pkg/action/validate.go#L34-L37
	appManagedByLabel              = "app.kubernetes.io/managed-by"
	appManagedByHelm               = "Helm"
	helmReleaseNameAnnotation      = "meta.helm.sh/release-name"
	helmReleaseNamespaceAnnotation = "meta.helm.sh/release-namespace"
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

				if managedBy, ok := c.Labels[appManagedByLabel]; !ok || managedBy != appManagedByHelm {
					c.Labels[appManagedByLabel] = appManagedByHelm
				}

				if c.Annotations == nil {
					c.Annotations = map[string]string{}
				}

				if releaseName, ok := c.Annotations[helmReleaseNameAnnotation]; !ok || releaseName != runtimeClassesChart {
					c.Annotations[helmReleaseNameAnnotation] = runtimeClassesChart
				}

				if namespace, ok := c.Annotations[helmReleaseNamespaceAnnotation]; !ok || namespace != metav1.NamespaceSystem {
					c.Annotations[helmReleaseNamespaceAnnotation] = metav1.NamespaceSystem
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
