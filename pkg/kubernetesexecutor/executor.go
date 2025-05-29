package kubernetesexecutor

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/loadbalancer"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/rke2/pkg/auth"
	"github.com/rancher/rke2/pkg/bootstrap"
	"github.com/rancher/rke2/pkg/controllers"
	"github.com/rancher/rke2/pkg/podtemplate"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/leader"
	"github.com/rancher/wrangler/v3/pkg/ratelimit"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	errNotImplemented = fmt.Errorf("not implemented")
	secretDirs        = []string{"cred", "etc", "tls"}
	clusterNameLabel  = version.Program + ".cattle.io/cluster-name"
)

type KubernetesConfig struct {
	podtemplate.Config

	IngressController string
	AuditPolicyFile   string
	PSAConfigFile     string

	KubeConfig string
	Name       string
	Domain     string

	apply            apply.Apply
	client           kubernetes.Interface
	namespace        string
	advertiseAddress string
	apiServerReady   <-chan struct{}
	etcdReady        chan struct{}
	criReady         chan struct{}
	config           *config.Control
	etcd             *etcd.ETCD
}

// explicit type checks
var _ executor.Executor = &KubernetesConfig{}
var _ controllers.Server = &KubernetesConfig{}

func (k *KubernetesConfig) Init(ctx context.Context) error {
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

	app, err := apply.NewForConfig(config)
	if err != nil {
		return err
	}
	k.apply = app.WithDynamicLookup()

	if err := k.extractClusterSecrets(ctx, secretDirs...); err != nil {
		return err
	}

	if err := k.applyAPIServerService(ctx); err != nil {
		return err
	}

	k.advertiseAddress, err = k.getAdvertiseAddress(ctx)
	if err != nil {
		return err
	}

	logrus.Infof("Initialized kubernetes client for cluster %s/%s at %s in domain %s", k.Name, k.namespace, k.advertiseAddress, k.Domain)

	// ensure certs are valid for services
	cmds.ServerConfig.TLSSan.Set(k.advertiseAddress)
	cmds.ServerConfig.TLSSan.Set(k.Name + "-etcd")
	cmds.ServerConfig.TLSSan.Set(k.Name + "-supervisor")
	cmds.ServerConfig.TLSSan.Set(k.Name + "-kube-apiserver")
	if k.Domain != "" {
		cmds.ServerConfig.TLSSan.Set(fmt.Sprintf("*.%s-etcd.%s.svc.%s", k.Name, k.namespace, k.Domain))
	}
	return nil
}

func (k *KubernetesConfig) APIServer(ctx context.Context, args []string) error {
	podArgs, err := k.Config.APIServer()
	if err != nil {
		return err
	}

	// start a loadbalancer for the apiserver that is backed by the Service,
	// since everything expects the apiserver to be available on servers at localhost
	url := fmt.Sprintf("https://%s-kube-apiserver:%d", k.Name, cmds.ServerConfig.APIServerPort)
	if _, err = loadbalancer.New(ctx, filepath.Join(k.DataDir, "agent"), loadbalancer.APIServerServiceName, url, cmds.ServerConfig.APIServerPort, false); err != nil {
		return err
	}

	auditLogFile := ""
	kubeletPreferredAddressTypesFound := false
	for i, arg := range args {
		switch name, value, _ := strings.Cut(arg, "="); name {
		case "--advertise-port", "--basic-auth-file":
			// This is an option k3s adds that does not exist upstream
			args = append(args[:i], args[i+1:]...)
		case "--audit-log-path":
			auditLogFile = value
		case "--kubelet-preferred-address-types":
			kubeletPreferredAddressTypesFound = true
		}
	}
	if !kubeletPreferredAddressTypesFound {
		args = append([]string{"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname"}, args...)
	}

	if k.CISMode && k.AuditPolicyFile == "" {
		k.AuditPolicyFile = podtemplate.DefaultAuditPolicyFile
	}

	if k.AuditPolicyFile != "" {
		if err := podtemplate.WriteDefaultPolicyFile(k.AuditPolicyFile); err != nil {
			return err
		}
		extraArgs := []string{
			"--audit-policy-file=" + k.AuditPolicyFile,
			"--audit-log-maxage=30",
			"--audit-log-maxbackup=10",
			"--audit-log-maxsize=100",
		}
		if auditLogFile == "" {
			auditLogFile = filepath.Join(k.DataDir, "server/logs/audit.log")
			extraArgs = append(extraArgs, "--audit-log-path="+auditLogFile)
		}
		args = append(extraArgs, args...)
	}

	// FIXME - figure out how to copy this into a secret and mount it somewhere other than /etc
	//args = append([]string{"--admission-control-config-file=" + k.PSAConfigFile}, args...)

	// set advertise address from apiserver service
	args = append(args, "--advertise-address="+k.advertiseAddress)

	files := []string{}
	excludeFiles := []string{}
	dirs := podtemplate.OnlyExisting(podtemplate.SSLDirs)
	if auditLogFile != "" && auditLogFile != "-" {
		dirs = append(dirs, filepath.Dir(auditLogFile))
		excludeFiles = append(excludeFiles, auditLogFile)
	}

	// FIXME - "server/cred/encryption-config.json" needs to be synced into secret when the content is updated
	podArgs.Args = args
	podArgs.Dirs = dirs
	podArgs.Files = files
	podArgs.ExcludeFiles = excludeFiles

	deployment, err := k.deployment(podArgs)
	if err != nil {
		return err
	}
	k.addSupervisorContainer(deployment)

	return podtemplate.After(k.ETCDReadyChan(), func() error {
		return k.apply.WithSetID(deployment.Name).ApplyObjects(deployment)
	})
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
	if k.client == nil {
		return fmt.Errorf("executor not initialized")
	}

	if err := bootstrap.UpdateManifests(k.Resolver, k.IngressController, nodeConfig, cfg); err != nil {
		logrus.Errorf("Failed to update manifests: %v", err)
	}

	leaderCh := make(chan struct{})
	go leader.RunOrDie(ctx, k.namespace, k.Name, k.client, func(ctx context.Context) {
		close(leaderCh)
	})
	<-leaderCh

	k.apiServerReady = util.APIServerReadyChan(ctx, nodeConfig.AgentConfig.KubeConfigK3sController, util.DefaultAPIServerReadyTimeout)
	k.etcdReady = make(chan struct{})
	k.criReady = make(chan struct{})
	close(k.criReady)

	return k.applyClusterSecrets(ctx, secretDirs...)
}

func (k *KubernetesConfig) CRI(ctx context.Context, node *config.Node) error {
	if node.ContainerRuntimeEndpoint != "/dev/null" {
		return errNotImplemented
	}
	return nil
}

func (k *KubernetesConfig) CRIReadyChan() <-chan struct{} {
	return k.criReady
}

func (k *KubernetesConfig) CloudControllerManager(ctx context.Context, ccmRBACReady <-chan struct{}, args []string) error {
	podArgs, err := k.Config.CloudControllerManager()
	if err != nil {
		return err
	}

	podArgs.Args = slices.DeleteFunc(args, func(arg string) bool { return strings.HasPrefix(arg, "--bind-address=") })
	podArgs.Dirs = podtemplate.OnlyExisting(podtemplate.SSLDirs)

	deployment, err := k.deployment(podArgs)
	if err != nil {
		return err
	}

	return podtemplate.After(ccmRBACReady, func() error {
		return k.apply.WithSetID(deployment.Name).ApplyObjects(deployment)
	})
}

func (k *KubernetesConfig) Containerd(ctx context.Context, node *config.Node) error {
	return errNotImplemented
}

func (k *KubernetesConfig) ControllerManager(ctx context.Context, args []string) error {
	podArgs, err := k.Config.ControllerManager()
	if err != nil {
		return err
	}

	podArgs.Args = slices.DeleteFunc(args, func(arg string) bool { return strings.HasPrefix(arg, "--bind-address=") })
	podArgs.Dirs = podtemplate.OnlyExisting(podtemplate.SSLDirs)

	deployment, err := k.deployment(podArgs)
	if err != nil {
		return err
	}

	return podtemplate.After(k.APIServerReadyChan(), func() error {
		return k.apply.WithSetID(deployment.Name).ApplyObjects(deployment)
	})
}

func (k *KubernetesConfig) CurrentETCDOptions() (executor.InitialOptions, error) {
	return executor.InitialOptions{}, nil
}

func (k *KubernetesConfig) Docker(ctx context.Context, node *config.Node) error {
	return errNotImplemented
}

func (k *KubernetesConfig) ETCD(ctx context.Context, args *executor.ETCDConfig, extraArgs []string, test executor.TestFunc) error {
	// start a loadbalancer for etcd that is backed by the Service,
	// since everything expects etcd to be available on servers at localhost:2379
	url := fmt.Sprintf("https://%s-etcd:2379", k.Name)
	if _, err := loadbalancer.New(ctx, filepath.Join(k.DataDir, "agent"), loadbalancer.ETCDServerServiceName, url, 2379, false); err != nil {
		return err
	}

	go func() {
		defer close(k.etcdReady)
		for {
			if err := test(ctx); err != nil {
				logrus.Infof("Failed to test etcd connection: %v", err)
			} else {
				logrus.Info("Connection to etcd is ready")
				return
			}

			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}()

	// nil args indicates a no-op start; all we need to do is wait for the test
	// func to indicate readiness and close the channel.
	if args == nil {
		return nil
	}

	podArgs, err := k.Config.ETCD()
	if err != nil {
		return err
	}

	confFile := filepath.Join(k.DataDir, "server", "db", "etcd", "config.$(POD_NAME)")

	podArgs.Args = []string{"--config-file=" + confFile}
	podArgs.ExcludeFiles = []string{confFile}
	podArgs.Files = []string{
		args.ServerTrust.CertFile,
		args.ServerTrust.KeyFile,
		args.ServerTrust.TrustedCAFile,
		args.PeerTrust.CertFile,
		args.PeerTrust.KeyFile,
		args.PeerTrust.TrustedCAFile,
	}
	sset, service, err := k.statefulSetWithService(podArgs)
	if err != nil {
		return err
	}

	secret, err := k.etcdConfigSecret(args, extraArgs, *sset.Spec.Replicas)
	if err != nil {
		return err
	}

	for i, c := range sset.Spec.Template.Spec.Containers {
		c.VolumeMounts = append(c.VolumeMounts,
			corev1.VolumeMount{
				Name:      "etcd-config",
				MountPath: filepath.Join(k.DataDir, "server", "db", "etcd"),
			},
			corev1.VolumeMount{
				Name:      "etcd-db",
				MountPath: filepath.Join(string(filepath.Separator), "db"),
			})
		sset.Spec.Template.Spec.Containers[i] = c
	}

	sset.Spec.Template.Spec.Volumes = append(sset.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: "etcd-config",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: k.Name + "-etcd-config",
				},
			},
		})

	sset.Spec.VolumeClaimTemplates = append(sset.Spec.VolumeClaimTemplates,
		corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: "etcd-db",
			}, Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("8G"),
					},
				},
			},
		})

	return k.apply.WithSetID(sset.Name).ApplyObjects(sset, service, secret)
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
	podArgs, err := k.Config.Scheduler()
	if err != nil {
		return err
	}

	podArgs.Dirs = podtemplate.OnlyExisting(podtemplate.SSLDirs)
	podArgs.Args = slices.DeleteFunc(args, func(arg string) bool { return strings.HasPrefix(arg, "--bind-address=") })

	deployment, err := k.deployment(podArgs)
	if err != nil {
		return err
	}

	return podtemplate.After(k.APIServerReadyChan(), func() error {
		return k.apply.WithSetID(deployment.Name).ApplyObjects(deployment)
	})
}

// getAdvertiseAddress waits for ClusterIP assignment for the apiserver service, this is what we will advertise to clients.
func (k *KubernetesConfig) getAdvertiseAddress(ctx context.Context) (string, error) {
	var advertiseAddress string
	serviceName := k.Name + "-kube-apiserver"
	err := wait.PollUntilContextCancel(ctx, time.Second, true, func(cxt context.Context) (bool, error) {
		s, err := k.client.CoreV1().Services(k.namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil {
			logrus.Infof("Waiting for create of Service %s", serviceName)
			return false, nil
		}
		switch s.Spec.Type {
		case corev1.ServiceTypeLoadBalancer:
			for _, i := range s.Status.LoadBalancer.Ingress {
				if i.IP != "" {
					advertiseAddress = i.IP
					return true, nil
				}
			}
			logrus.Infof("Waiting for LoadBalancer Ingress IP assignment for service %s", serviceName)
			return false, nil
		case corev1.ServiceTypeClusterIP:
			if s.Spec.ClusterIP == "" || s.Spec.ClusterIP == "None" {
				logrus.Infof("Waiting for ClusterIP assignment for Service %s", serviceName)
				return false, nil
			}
			advertiseAddress = s.Spec.ClusterIP
			return true, nil
		default:
			return false, fmt.Errorf("unsupported Service Type %s", s.Spec.Type)
		}
	})
	return advertiseAddress, err
}
