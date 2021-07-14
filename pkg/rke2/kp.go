package rke2

import (
	"context"

	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

const kubeProxyChart = "rke2-kube-proxy"

// setKubeProxyDisabled determines if a cluster already has kube proxy deployed as a chart, if so
// disables running kubeproxy as a static pod.
func setKubeProxyDisabled(clx *cli.Context, cfg *cmds.Server) func(context.Context, <-chan struct{}, string) error {
	return func(ctx context.Context, apiServerReady <-chan struct{}, kubeConfigAdmin string) error {
		go func() {
			<-apiServerReady
			logrus.Info("Checking for kube proxy as a chart")

			restConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigAdmin)
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
					clx.Set("disable", kubeProxyChart)
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
