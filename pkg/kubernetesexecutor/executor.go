package kubernetesexecutor

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/rancher/rke2/pkg/auth"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/ratelimit"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var errNotImplemented = fmt.Errorf("not implemented")

type KubernetesConfig struct {
	DataDir    string
	KubeConfig string
	Name       string

	apply          apply.Apply
	client         *kubernetes.Clientset
	namespace      string
	apiServerReady <-chan struct{}
	etcdReady      chan struct{}
	criReady       chan struct{}
}

func (k *KubernetesConfig) APIServer(ctx context.Context, args []string) error {
	return nil
}

func (k *KubernetesConfig) APIServerHandlers(ctx context.Context) (authenticator.Request, http.Handler, error) {
	<-k.APIServerReadyChan()
	kubeConfigAPIServer := filepath.Join(k.DataDir, "server", "cred", "api-server.kubeconfig")
	tokenauth, err := auth.BootstrapTokenAuthenticator(ctx, kubeConfigAPIServer)
	return tokenauth, http.NotFoundHandler(), err
}

func (k *KubernetesConfig) APIServerReadyChan() <-chan struct{} {
	return k.apiServerReady
}

func (k *KubernetesConfig) Bootstrap(ctx context.Context, nodeConfig *config.Node, cfg cmds.Agent) error {
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: k.KubeConfig},
		&clientcmd.ConfigOverrides{},
	)

	var err error
	k.namespace, _, err = loader.Namespace()
	if err != nil {
		return err
	}

	config, err := loader.ClientConfig()
	if err != nil {
		return err
	}

	config.Timeout = 15 * time.Minute
	config.RateLimiter = ratelimit.None

	k.client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	k.apply, err = apply.NewForConfig(config)
	if err != nil {
		return err
	}

	k.apiServerReady = util.APIServerReadyChan(ctx, nodeConfig.AgentConfig.KubeConfigK3sController, util.DefaultAPIServerReadyTimeout)
	k.etcdReady = make(chan struct{})
	k.criReady = make(chan struct{})
	defer close(k.etcdReady)
	defer close(k.criReady)

	return nil
}

func (k *KubernetesConfig) CRI(ctx context.Context, node *config.Node) error {
	return errNotImplemented
}

func (k *KubernetesConfig) CRIReadyChan() <-chan struct{} {
	return k.criReady
}

func (k *KubernetesConfig) CloudControllerManager(ctx context.Context, ccmRBACReady <-chan struct{}, args []string) error {
	return nil
}

func (k *KubernetesConfig) Containerd(ctx context.Context, node *config.Node) error {
	return errNotImplemented
}

func (k *KubernetesConfig) ControllerManager(ctx context.Context, args []string) error {
	return nil
}

func (k *KubernetesConfig) CurrentETCDOptions() (executor.InitialOptions, error) {
	return executor.InitialOptions{}, nil
}

func (k *KubernetesConfig) Docker(ctx context.Context, node *config.Node) error {
	return errNotImplemented
}

func (k *KubernetesConfig) ETCD(ctx context.Context, args *executor.ETCDConfig, extraArgs []string, test executor.TestFunc) error {
	return errNotImplemented
}

func (k *KubernetesConfig) ETCDReadyChan() <-chan struct{} {
	return k.etcdReady
}

func (k *KubernetesConfig) KubeProxy(ctx context.Context, args []string) error {
	return errNotImplemented
}

func (k *KubernetesConfig) Kubelet(ctx context.Context, args []string) error {
	return errNotImplemented
}

func (k *KubernetesConfig) Scheduler(ctx context.Context, nodeReady <-chan struct{}, args []string) error {
	return nil
}
