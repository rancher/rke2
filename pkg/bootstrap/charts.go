package bootstrap

import (
	"context"

	helmv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	helm "github.com/k3s-io/helm-controller/pkg/generated/clientset/versioned/typed/helm.cattle.io/v1"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/util/errors"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

// ListHelmCharts waits for the apiserver and RBAC to be ready, then returns a list of all charts in the kube-system namespace.
func ListHelmCharts(ctx context.Context, kubeConfig string) (*helmv1.HelmChartList, error) {
	select {
	case <-executor.APIServerReadyChan():
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if err := util.WaitForRBACReady(ctx, kubeConfig, util.DefaultAPIServerReadyTimeout, authorizationv1.ResourceAttributes{
		Namespace: metav1.NamespaceSystem,
		Verb:      "list",
		Group:     helmv1.SchemeGroupVersion.Group,
		Resource:  helmv1.HelmChartResourceName,
	}, ""); err != nil {
		return nil, errors.WithMessage(err, "failed to wait for RBAC")
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return nil, err
	}

	hc, err := helm.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return hc.HelmCharts(metav1.NamespaceSystem).List(ctx, metav1.ListOptions{})
}
