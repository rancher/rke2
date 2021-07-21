package rke2

import (
	"context"
	"sync"

	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

const kubeProxyChart = "rke2-kube-proxy"

// setKubeProxyDisabled determines if a cluster already has kube proxy deployed as a chart, if so
// disables running kubeproxy as a static pod.
func setKubeProxyDisabled(cfg *cmds.Server) cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		go func() {
			defer wg.Done()
			<-args.APIServerReady
			logrus.Info("Checking for kube proxy as a chart")

			restConfig, err := clientcmd.BuildConfigFromFlags("", args.KubeConfigAdmin)
			if err != nil {
				logrus.Fatalf("kp: new k8s client: %s", err.Error())
			}

			hc, err := helm.NewFactoryFromConfig(restConfig)
			if err != nil {
				logrus.Fatalf("kp: new helm client: %s", err.Error())
			}
			if _, err := hc.Helm().V1().HelmChartConfig().Get(metav1.NamespaceSystem, kubeProxyChart, metav1.GetOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					logrus.Infof("%[1]s HelmChartConfig not found, disabling %[1]s", kubeProxyChart)
					args.Skips[kubeProxyChart] = true
					args.Disables[kubeProxyChart] = true
					return
				}
				logrus.WithError(err).Fatalf("kp: failed to check for %s HelmChartConfig", kubeProxyChart)
				return
			}
			logrus.Infof("%s HelmChartConfig found, disabling embedded kube-proxy", kubeProxyChart)
			cfg.DisableKubeProxy = true
		}()
		return nil
	}
}
