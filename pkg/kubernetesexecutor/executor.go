package kubernetesexecutor

import (
	"context"
	"fmt"
	"net/http"
	"os"
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
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/podexecutor"
	"github.com/rancher/rke2/pkg/staticpod"
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
	podexecutor.ControlPlaneResources
	podexecutor.ControlPlaneEnv
	podexecutor.ControlPlaneProbeConfs
	Resolver *images.Resolver

	DataDir           string
	IngressController string
	AuditPolicyFile   string
	PSAConfigFile     string

	KubeConfig string
	Name       string
	Domain     string
	CISMode    bool

	apply          apply.Apply
	client         kubernetes.Interface
	namespace      string
	apiServerReady <-chan struct{}
	etcdReady      chan struct{}
	criReady       chan struct{}
	config         *config.Control
	etcd           *etcd.ETCD
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

	// ensure certs are valid for services
	cmds.ServerConfig.TLSSan.Set(k.Name + "-etcd")
	cmds.ServerConfig.TLSSan.Set(k.Name + "-supervisor")
	cmds.ServerConfig.TLSSan.Set(k.Name + "-kube-apiserver")
	if k.Domain != "" {
		cmds.ServerConfig.TLSSan.Set(fmt.Sprintf("*.%s-etcd.%s.svc.%s", k.Name, k.namespace, k.Domain))
	}

	logrus.Infof("Initialized kubernetes client for cluster %s/%s in domain %s", k.Name, k.namespace, k.Domain)
	return k.extractClusterSecrets(ctx, secretDirs...)
}

func (k *KubernetesConfig) APIServer(ctx context.Context, args []string) error {
	image, err := k.Resolver.GetReference(images.KubeAPIServer)
	if err != nil {
		return err
	}

	// start a loadbalancer for the apiserver that is backed by the Service,
	// since everything expects the apiserver to be available on servers at localhost:6443
	url := fmt.Sprintf("https://%s-kube-apiserver:6443", k.Name)
	if _, err = loadbalancer.New(ctx, filepath.Join(k.DataDir, "agent"), loadbalancer.APIServerServiceName, url, 6443, false); err != nil {
		return err
	}

	advertiseAddress := ""
	if advertiseEnv := os.Getenv(version.ProgramUpper + "_ADVERTISE_ADDRESS"); advertiseEnv != "" {
		if !slices.Contains(cmds.ServerConfig.TLSSan.Value(), advertiseEnv) {
			return fmt.Errorf("Advertised address not in TLS SAN list")
		}
		advertiseAddress = advertiseEnv
	} else {
		advertiseAddress, err = k.getAdvertiseAddress(ctx)
		if err != nil {
			return err
		}
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
		k.AuditPolicyFile = podexecutor.DefaultAuditPolicyFile
	}

	if k.AuditPolicyFile != "" {
		if err := podexecutor.WriteDefaultPolicyFile(k.AuditPolicyFile); err != nil {
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

	// set advertise address from apiserver service or configured value
	args = append(args, "--advertise-address="+advertiseAddress)

	files := []string{}
	excludeFiles := []string{}
	dirs := podexecutor.OnlyExisting(podexecutor.SSLDirs)
	if auditLogFile != "" && auditLogFile != "-" {
		dirs = append(dirs, filepath.Dir(auditLogFile))
		excludeFiles = append(excludeFiles, auditLogFile)
	}

	// FIXME - "server/cred/encryption-config.json" needs to be synced into secret when the content is updated

	podArgs := staticpod.Args{
		Command:       "kube-apiserver",
		Args:          args,
		Image:         image,
		Dirs:          dirs,
		CISMode:       k.CISMode,
		Ports:         []corev1.ContainerPort{{Name: "https", Protocol: corev1.ProtocolTCP, ContainerPort: 6443}},
		CPURequest:    k.ControlPlaneResources.KubeAPIServerCPURequest,
		CPULimit:      k.ControlPlaneResources.KubeAPIServerCPULimit,
		MemoryRequest: k.ControlPlaneResources.KubeAPIServerMemoryRequest,
		MemoryLimit:   k.ControlPlaneResources.KubeAPIServerMemoryLimit,
		ExtraEnv:      k.ControlPlaneEnv.KubeAPIServer,
		ProbeConfs:    k.ControlPlaneProbeConfs.KubeAPIServer,
		Files:         files,
		ExcludeFiles:  excludeFiles,
		StartupExec: []string{
			"kubectl",
			"get",
			"--server=https://localhost:6443/",
			"--client-certificate=" + k.DataDir + "/server/tls/client-kube-apiserver.crt",
			"--client-key=" + k.DataDir + "/server/tls/client-kube-apiserver.key",
			"--certificate-authority=" + k.DataDir + "/server/tls/server-ca.crt",
			"--raw=/livez",
		},
		HealthExec: []string{
			"kubectl",
			"get",
			"--server=https://localhost:6443/",
			"--client-certificate=" + k.DataDir + "/server/tls/client-kube-apiserver.crt",
			"--client-key=" + k.DataDir + "/server/tls/client-kube-apiserver.key",
			"--certificate-authority=" + k.DataDir + "/server/tls/server-ca.crt",
			"--raw=/livez",
		},
		ReadyExec: []string{
			"kubectl",
			"get",
			"--server=https://localhost:6443/",
			"--client-certificate=" + k.DataDir + "/server/tls/client-kube-apiserver.crt",
			"--client-key=" + k.DataDir + "/server/tls/client-kube-apiserver.key",
			"--certificate-authority=" + k.DataDir + "/server/tls/server-ca.crt",
			"--raw=/readyz",
		},
	}

	deployment, err := k.deployment(podArgs)
	if err != nil {
		return err
	}
	k.addSupervisorContainer(deployment)
	return k.apply.WithSetID(deployment.Name).ApplyObjects(deployment)
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

	return k.applyClusterResources(ctx, secretDirs...)
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
	image, err := k.Resolver.GetReference(images.CloudControllerManager)
	if err != nil {
		return err
	}

	files := []string{}
	excludeFiles := []string{}
	dirs := podexecutor.OnlyExisting(podexecutor.SSLDirs)
	args = slices.DeleteFunc(args, func(arg string) bool { return strings.HasPrefix(arg, "--bind-address=") })

	podArgs := staticpod.Args{
		Command:       "cloud-controller-manager",
		Args:          args,
		Image:         image,
		Dirs:          dirs,
		CISMode:       k.CISMode,
		HealthPort:    10258,
		HealthScheme:  "HTTPS",
		HealthPath:    "/healthz",
		StartupPort:   10258,
		StartupScheme: "HTTPS",
		StartupPath:   "/healthz",
		CPURequest:    k.ControlPlaneResources.CloudControllerManagerCPURequest,
		CPULimit:      k.ControlPlaneResources.CloudControllerManagerCPULimit,
		MemoryRequest: k.ControlPlaneResources.CloudControllerManagerMemoryRequest,
		MemoryLimit:   k.ControlPlaneResources.CloudControllerManagerMemoryLimit,
		ExtraEnv:      k.ControlPlaneEnv.CloudControllerManager,
		ProbeConfs:    k.ControlPlaneProbeConfs.CloudControllerManager,
		Files:         files,
		ExcludeFiles:  excludeFiles,
	}

	deployment, err := k.deployment(podArgs)
	if err != nil {
		return err
	}

	return podexecutor.After(ccmRBACReady, func() error {
		return k.apply.WithSetID(deployment.Name).ApplyObjects(deployment)
	})
}

func (k *KubernetesConfig) Containerd(ctx context.Context, node *config.Node) error {
	return errNotImplemented
}

func (k *KubernetesConfig) ControllerManager(ctx context.Context, args []string) error {
	image, err := k.Resolver.GetReference(images.KubeControllerManager)
	if err != nil {
		return err
	}

	files := []string{}
	excludeFiles := []string{}
	dirs := podexecutor.OnlyExisting(podexecutor.SSLDirs)
	args = slices.DeleteFunc(args, func(arg string) bool { return strings.HasPrefix(arg, "--bind-address=") })

	podArgs := staticpod.Args{
		Command:       "kube-controller-manager",
		Args:          args,
		Image:         image,
		Dirs:          dirs,
		CISMode:       k.CISMode,
		HealthPort:    10257,
		HealthScheme:  "HTTPS",
		HealthPath:    "/healthz",
		StartupPort:   10257,
		StartupScheme: "HTTPS",
		StartupPath:   "/healthz",
		CPURequest:    k.ControlPlaneResources.KubeControllerManagerCPURequest,
		CPULimit:      k.ControlPlaneResources.KubeControllerManagerCPULimit,
		MemoryRequest: k.ControlPlaneResources.KubeControllerManagerMemoryRequest,
		MemoryLimit:   k.ControlPlaneResources.KubeControllerManagerMemoryLimit,
		ExtraEnv:      k.ControlPlaneEnv.KubeControllerManager,
		ProbeConfs:    k.ControlPlaneProbeConfs.KubeControllerManager,
		Files:         files,
		ExcludeFiles:  excludeFiles,
	}

	deployment, err := k.deployment(podArgs)
	if err != nil {
		return err
	}

	return podexecutor.After(k.APIServerReadyChan(), func() error {
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

	image, err := k.Resolver.GetReference(images.ETCD)
	if err != nil {
		return err
	}

	confFile := filepath.Join(k.DataDir, "server", "db", "etcd", "config.$(POD_NAME)")

	podArgs := staticpod.Args{
		Command:      "etcd",
		Args:         []string{"--config-file=" + confFile},
		Image:        image,
		ExcludeFiles: []string{confFile},
		Files: []string{
			args.ServerTrust.CertFile,
			args.ServerTrust.KeyFile,
			args.ServerTrust.TrustedCAFile,
			args.PeerTrust.CertFile,
			args.PeerTrust.KeyFile,
			args.PeerTrust.TrustedCAFile,
		},
		CISMode: k.CISMode,
		Ports: []corev1.ContainerPort{
			{Name: "client", Protocol: corev1.ProtocolTCP, ContainerPort: 2379},
			{Name: "peer", Protocol: corev1.ProtocolTCP, ContainerPort: 2380},
			{Name: "metrics", Protocol: corev1.ProtocolTCP, ContainerPort: 2381},
		},
		HealthPort:    2381,
		HealthPath:    "/health?serializable=true",
		HealthScheme:  "HTTP",
		CPURequest:    k.ControlPlaneResources.EtcdCPURequest,
		CPULimit:      k.ControlPlaneResources.EtcdCPULimit,
		MemoryRequest: k.ControlPlaneResources.EtcdMemoryRequest,
		MemoryLimit:   k.ControlPlaneResources.EtcdMemoryLimit,
		ExtraEnv:      k.ControlPlaneEnv.Etcd,
		ProbeConfs:    k.ControlPlaneProbeConfs.Etcd,
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
	image, err := k.Resolver.GetReference(images.KubeScheduler)
	if err != nil {
		return err
	}

	files := []string{}
	excludeFiles := []string{}
	dirs := podexecutor.OnlyExisting(podexecutor.SSLDirs)
	args = slices.DeleteFunc(args, func(arg string) bool { return strings.HasPrefix(arg, "--bind-address=") })

	podArgs := staticpod.Args{
		Command:       "kube-scheduler",
		Args:          args,
		Image:         image,
		Dirs:          dirs,
		CISMode:       k.CISMode,
		HealthPort:    10259,
		HealthScheme:  "HTTPS",
		ReadyPort:     10259,
		ReadyScheme:   "HTTPS",
		ReadyPath:     "/readyz",
		StartupPort:   10259,
		StartupScheme: "HTTPS",
		CPURequest:    k.ControlPlaneResources.KubeSchedulerCPURequest,
		CPULimit:      k.ControlPlaneResources.KubeSchedulerCPULimit,
		MemoryRequest: k.ControlPlaneResources.KubeSchedulerMemoryRequest,
		MemoryLimit:   k.ControlPlaneResources.KubeSchedulerMemoryLimit,
		ExtraEnv:      k.ControlPlaneEnv.KubeScheduler,
		ProbeConfs:    k.ControlPlaneProbeConfs.KubeScheduler,
		Files:         files,
		ExcludeFiles:  excludeFiles,
	}

	deployment, err := k.deployment(podArgs)
	if err != nil {
		return err
	}

	return podexecutor.After(k.APIServerReadyChan(), func() error {
		return k.apply.WithSetID(deployment.Name).ApplyObjects(deployment)
	})
}

// getAdvertiseAddress waits for ClusterIP assignment for the apiserver service, this is what we will advertise to clients.
func (k *KubernetesConfig) getAdvertiseAddress(ctx context.Context) (string, error) {
	// FIXME - use LoadBalancer IP if available
	var advertiseAddress string
	err := wait.PollUntilContextCancel(ctx, time.Second, true, func(cxt context.Context) (bool, error) {
		serviceName := k.Name + "-kube-apiserver"
		s, err := k.client.CoreV1().Services(k.namespace).Get(ctx, serviceName, metav1.GetOptions{})
		if err != nil {
			logrus.Infof("Waiting for create of Service %s", serviceName)
			return false, nil
		}
		if s.Spec.ClusterIP == "" || s.Spec.ClusterIP == "None" {
			logrus.Infof("Waiting for ClusterIP assignment for Service %s", serviceName)
			return false, nil
		}
		advertiseAddress = s.Spec.ClusterIP
		return true, nil
	})
	return advertiseAddress, err
}
