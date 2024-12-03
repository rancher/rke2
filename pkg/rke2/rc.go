package rke2

import (
	"context"
	"sync"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const runtimeClassesChart = "rke2-runtimeclasses"

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

			config, err := clientcmd.BuildConfigFromFlags("", args.KubeConfigSupervisor)
			if err != nil {
				logrus.Fatalf("runtimes: new k8s restConfig: %v", err)
			}

			client, err := kubernetes.NewForConfig(config)
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
					labels := make(map[string]string)
					c.SetLabels(labels)
				}

				if managedBy, ok := c.Labels["app.kubernetes.io/managed-by"]; !ok || managedBy != "Helm" {
					c.Labels["app.kubernetes.io/managed-by"] = "Helm"
				}

				if c.Annotations == nil {
					annotations := make(map[string]string)
					c.SetAnnotations(annotations)
				}

				if releaseName, ok := c.Annotations["meta.helm.sh/release-name"]; !ok || releaseName != runtimeClassesChart {
					c.Annotations["meta.helm.sh/release-name"] = runtimeClassesChart
				}

				if namespace, ok := c.Annotations["meta.helm.sh/release-namespace"]; !ok || namespace != "kube-system" {
					c.Annotations["meta.helm.sh/release-namespace"] = "kube-system"
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
