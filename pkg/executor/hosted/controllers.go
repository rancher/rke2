package hosted

import (
	"context"
	"path/filepath"
	"time"

	helmchart "github.com/k3s-io/helm-controller/pkg/controllers/chart"
	helmcommon "github.com/k3s-io/helm-controller/pkg/controllers/common"
	helmcrds "github.com/k3s-io/helm-controller/pkg/crds"
	"github.com/k3s-io/k3s/pkg/server"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/crd"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

func (h *HostedConfig) LeaderControllers() server.CustomControllers {
	return server.CustomControllers{
		h.helmController,
	}
}

func (h *HostedConfig) Controllers() server.CustomControllers {
	return nil
}

// This is almost entirely the same thing as what k3s does if we leave the built-in
// helm controller enabled, but we add an injector to the Apply controller
// to remove the control-plane nodeselector, as there will not be any control-plane nodes.
func (h *HostedConfig) helmController(ctx context.Context, sc *server.Context) error {
	cfg := filepath.Join(h.DataDir, "server", "cred", "supervisor.kubeconfig")
	config, err := util.GetRESTConfig(cfg)
	if err != nil {
		return err
	}
	config.UserAgent = util.GetUserAgent(helmcommon.Name)

	client, err := clientset.NewForConfig(config)
	if err != nil {
		return err
	}

	crds, err := helmcrds.List()
	if err != nil {
		return err
	}

	if err := crd.BatchCreateCRDs(ctx, client.ApiextensionsV1().CustomResourceDefinitions(), nil, time.Minute, crds); err != nil {
		return err
	}

	k8s, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	apply := apply.New(k8s, apply.NewClientFactory(config)).WithInjector(func(objs []runtime.Object) ([]runtime.Object, error) {
		for i, obj := range objs {
			if job, ok := obj.(*batchv1.Job); ok {
				delete(job.Spec.Template.Spec.NodeSelector, helmchart.LabelNodeRolePrefix+helmchart.LabelControlPlaneSuffix)
				objs[i] = job
			}
		}
		return objs, nil
	}).WithDynamicLookup().WithSetOwnerReference(false, false)

	helm := sc.Helm.WithAgent(config.UserAgent)
	batch := sc.Batch.WithAgent(config.UserAgent)
	auth := sc.Auth.WithAgent(config.UserAgent)
	core := sc.Core.WithAgent(config.UserAgent)
	helmchart.Register(ctx,
		metav1.NamespaceAll,
		helmcommon.Name,
		"cluster-admin",
		"6443",
		k8s,
		apply,
		util.BuildControllerEventRecorder(k8s, helmcommon.Name, metav1.NamespaceAll),
		helm.V1().HelmChart(),
		helm.V1().HelmChart().Cache(),
		helm.V1().HelmChartConfig(),
		helm.V1().HelmChartConfig().Cache(),
		batch.V1().Job(),
		batch.V1().Job().Cache(),
		auth.V1().ClusterRoleBinding(),
		core.V1().ServiceAccount(),
		core.V1().ConfigMap(),
		core.V1().Secret(),
		core.V1().Secret().Cache(),
	)
	return nil
}
