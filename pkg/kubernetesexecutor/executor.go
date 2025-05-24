package kubernetesexecutor

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/loadbalancer"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/rke2/pkg/auth"
	"github.com/rancher/rke2/pkg/bootstrap"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/podexecutor"
	"github.com/rancher/rke2/pkg/staticpod"
	"github.com/rancher/wrangler/v3/pkg/apply"
	"github.com/rancher/wrangler/v3/pkg/leader"
	"github.com/rancher/wrangler/v3/pkg/ratelimit"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
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
	CISMode    bool

	apply          apply.Apply
	client         kubernetes.Interface
	loadbalancer   *loadbalancer.LoadBalancer
	namespace      string
	apiServerReady <-chan struct{}
	etcdReady      chan struct{}
	criReady       chan struct{}
}

func (k *KubernetesConfig) APIServer(ctx context.Context, args []string) error {
	image, err := k.Resolver.GetReference(images.KubeAPIServer)
	if err != nil {
		return err
	}

	// start a loadbalancer for the apiserver that is backed by the Service,
	// since everything expects the apiserver to be available on servers at localhost:6443
	url := fmt.Sprintf("https://%s-kube-apiserver:6443", k.Name)
	k.loadbalancer, err = loadbalancer.New(ctx, filepath.Join(k.DataDir, "agent"), loadbalancer.APIServerServiceName, url, 6443, false)
	if err != nil {
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
	return k.apply.WithSetID(k.Name + "-" + deployment.Name).ApplyObjects(deployment)
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

	app, err := apply.NewForConfig(config)
	if err != nil {
		return err
	}

	if err := bootstrap.UpdateManifests(k.Resolver, k.IngressController, nodeConfig, cfg); err != nil {
		logrus.Errorf("Failed to update manifests: %v", err)
	}

	leaderCh := make(chan struct{})
	go leader.RunOrDie(ctx, k.namespace, k.Name, k.client, func(ctx context.Context) {
		close(leaderCh)
	})
	<-leaderCh

	k.apply = app.WithDynamicLookup()
	k.apiServerReady = util.APIServerReadyChan(ctx, nodeConfig.AgentConfig.KubeConfigK3sController, util.DefaultAPIServerReadyTimeout)
	k.etcdReady = make(chan struct{})
	k.criReady = make(chan struct{})
	close(k.etcdReady)
	close(k.criReady)

	return k.applyClusterResources(ctx)
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

	podArgs := staticpod.Args{
		Command:       "cloud-controller-manager",
		Args:          args,
		Image:         image,
		Dirs:          dirs,
		CISMode:       k.CISMode,
		HealthPort:    10258,
		HealthProto:   "HTTPS",
		HealthPath:    "/healthz",
		StartupPort:   10258,
		StartupProto:  "HTTPS",
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
		return k.apply.WithSetID(k.Name + "-" + deployment.Name).ApplyObjects(deployment)
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

	podArgs := staticpod.Args{
		Command:       "kube-controller-manager",
		Args:          args,
		Image:         image,
		Dirs:          dirs,
		CISMode:       k.CISMode,
		HealthPort:    10257,
		HealthProto:   "HTTPS",
		HealthPath:    "/healthz",
		StartupPort:   10257,
		StartupProto:  "HTTPS",
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
		return k.apply.WithSetID(k.Name + "-" + deployment.Name).ApplyObjects(deployment)
	})
}

func (k *KubernetesConfig) CurrentETCDOptions() (executor.InitialOptions, error) {
	return executor.InitialOptions{}, nil
}

func (k *KubernetesConfig) Docker(ctx context.Context, node *config.Node) error {
	return errNotImplemented
}

func (k *KubernetesConfig) ETCD(ctx context.Context, args *executor.ETCDConfig, extraArgs []string, test executor.TestFunc) error {
	// TODO: manage a single etcd pod per controller replica?
	// Need to figure out how to bootstrap, as we currently expect the supervisor to have access to the etcd
	// database files and pull stuff out of them with the embedded executor.
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
	image, err := k.Resolver.GetReference(images.KubeScheduler)
	if err != nil {
		return err
	}

	files := []string{}
	excludeFiles := []string{}
	dirs := podexecutor.OnlyExisting(podexecutor.SSLDirs)

	podArgs := staticpod.Args{
		Command:       "kube-scheduler",
		Args:          args,
		Image:         image,
		Dirs:          dirs,
		CISMode:       k.CISMode,
		HealthPort:    10259,
		HealthProto:   "HTTPS",
		ReadyPort:     10259,
		ReadyProto:    "HTTPS",
		ReadyPath:     "/readyz",
		StartupPort:   10259,
		StartupProto:  "HTTPS",
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
		return k.apply.WithSetID(k.Name + "-" + deployment.Name).ApplyObjects(deployment)
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

// applyClusterResources creates secrets containing the contents of the
// server tls, etc, and cred directories. This content is mounted from the
// host by static pods, but is packaged into a Secret for use by the Deployment pods.
// It also creates a Service for the apiserver.
func (k *KubernetesConfig) applyClusterResources(ctx context.Context) error {
	objs := []runtime.Object{}
	serviceType := corev1.ServiceTypeClusterIP
	if typeEnv := os.Getenv(version.ProgramUpper + "_CLUSTER_SERVICETYPE"); typeEnv != "" {
		serviceType = corev1.ServiceType(typeEnv)
	}

	for _, dir := range secretDirs {
		data, err := k.bytesFromDir(filepath.Join(k.DataDir, "server", dir))
		if err != nil {
			return err
		}
		objs = append(objs, &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      k.Name + "-server-" + dir,
				Namespace: k.namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		})
	}
	objs = append(objs, &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.Name + "-kube-apiserver",
			Namespace: k.namespace,
			Labels: map[string]string{
				clusterNameLabel: k.Name,
				"component":      "kube-apiserver",
				"tier":           "control-plane",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType,
			Selector: map[string]string{
				clusterNameLabel: k.Name,
				"component":      "kube-apiserver",
				"tier":           "control-plane",
			},
			Ports: []corev1.ServicePort{
				{Name: "https", Port: 6443, TargetPort: intstr.FromInt(6443)},
				{Name: "supervisor", Port: 9345, TargetPort: intstr.FromInt(9345)},
			},
		},
	})

	return k.apply.WithSetID(k.Name + "-controlplane").ApplyObjects(objs...)
}

// deployment returns a Deployment for the provided staticpod Args.
// This is essentially a wrapper around the normal static pod manifest.
func (k *KubernetesConfig) deployment(args staticpod.Args) (*appsv1.Deployment, error) {
	files, err := staticpod.ReadFiles(args.Args, args.ExcludeFiles)
	if err != nil {
		return nil, err
	}

	// TODO Check to make sure we aren't double mounting directories and the files in those directories

	args.Files = append(args.Files, files...)
	pod, err := staticpod.Pod(args)
	if err != nil {
		return nil, err
	}

	// Fix up name/namespace/labels and disable unnecessary things
	pod.Labels[clusterNameLabel] = k.Name
	pod.Name = fmt.Sprintf("%s-%s", k.Name, pod.Name)
	pod.Namespace = k.namespace
	pod.Spec.HostNetwork = false
	pod.Spec.AutomountServiceAccountToken = ptr.To(false)

	// Convert individual HostPath mounts to Secret volumes,
	// grouped by top-level directory.
	volumesToRemove := sets.Set[string]{}
	pathGroups := map[string]sets.Set[string]{}

	for _, dir := range secretDirs {
		path := filepath.Join(k.DataDir, "server", dir) + string(filepath.Separator)
		pathGroups[path] = sets.Set[string]{}
	}

	// Remove VolumeMounts from Containers for anything that will be mounted from a Secret
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].VolumeMounts = slices.DeleteFunc(pod.Spec.Containers[i].VolumeMounts, func(vm corev1.VolumeMount) bool {
			for prefix, group := range pathGroups {
				if strings.HasPrefix(vm.MountPath, prefix) {
					group.Insert(vm.MountPath)
					volumesToRemove.Insert(vm.Name)
					return true
				}
			}
			return false
		})
		for prefix := range pathGroups {
			pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
				MountPath: prefix,
				Name:      "secret-server-" + filepath.Base(prefix),
				ReadOnly:  true,
			})
		}
	}

	// Remove Volumes for removed VolumeMounts
	pod.Spec.Volumes = slices.DeleteFunc(pod.Spec.Volumes, func(v corev1.Volume) bool {
		return volumesToRemove.Has(v.Name)
	})

	// Add Volumes for Secrets
	for prefix, group := range pathGroups {
		volume := corev1.Volume{
			Name: "secret-server-" + filepath.Base(prefix),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: fmt.Sprintf("%s-server-%s", k.Name, filepath.Base(prefix)),
				},
			},
		}
		for _, path := range group.UnsortedList() {
			path = strings.TrimPrefix(path, prefix)
			item := corev1.KeyToPath{Path: path}
			path = strings.ReplaceAll(path, string(filepath.Separator), "_")
			item.Key = path
			volume.Secret.Items = append(volume.Secret.Items, item)
		}
		pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
	}

	// Add anti-affinity on node hostname
	pod.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				corev1.WeightedPodAffinityTerm{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: corev1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: pod.Labels,
						},
					},
				},
			},
		},
	}

	return deploymentForPod(pod), nil
}

// deploymentForPod creates a Deployment with Name, LabelSelector, and Pod from the Pod
func deploymentForPod(pod *corev1.Pod) *appsv1.Deployment {
	replicas := 1
	maxUnavailable := 1

	if replicasEnv := os.Getenv(version.ProgramUpper + "_CLUSTER_REPLICAS"); replicasEnv != "" {
		if r, err := strconv.Atoi(replicasEnv); err == nil && r >= 0 {
			replicas = r
		}
	}

	if replicas > 1 {
		maxUnavailable = replicas - 1
	}

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels:    pod.Labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             ptr.To(int32(replicas)),
			RevisionHistoryLimit: ptr.To(int32(0)),
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       ptr.To(intstr.FromInt(1)),
					MaxUnavailable: ptr.To(intstr.FromInt(maxUnavailable)),
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: pod.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: pod.Labels,
				},
				Spec: pod.Spec,
			},
		},
	}
}

func (k *KubernetesConfig) addSupervisorContainer(d *appsv1.Deployment) {
	envPrefix := strings.ToUpper(strings.ReplaceAll(k.Name+"-supervisor", "-", "_"))
	args := []string{
		"iptables -t nat -I PREROUTING -p tcp --dport $(" + envPrefix + "_SERVICE_PORT) -j DNAT --to $(" + envPrefix + "_SERVICE_HOST):$(" + envPrefix + "_SERVICE_PORT)",
		"iptables -t nat -I POSTROUTING -d $(" + envPrefix + "_SERVICE_HOST)/32 -p tcp -j MASQUERADE",
		"mkfifo /pause",
		"</pause",
	}
	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, corev1.Container{
		Name:    "supervisor",
		Image:   d.Spec.Template.Spec.Containers[0].Image,
		Command: []string{"/bin/sh", "-xec"},
		Args:    []string{strings.Join(args, "\n")},
		Ports:   []corev1.ContainerPort{{Name: "supervisor", Protocol: corev1.ProtocolTCP, ContainerPort: 9345}},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"NET_ADMIN",
				},
			},
		},
	})

	if d.Spec.Template.Spec.SecurityContext == nil {
		d.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	d.Spec.Template.Spec.SecurityContext.Sysctls = append(d.Spec.Template.Spec.SecurityContext.Sysctls, corev1.Sysctl{Name: "net.ipv4.ip_forward", Value: "1"})
}

// bytesFromDir returns a map of paths to bytes for normal files under a directory.
// Path separators are converted to underscores, so that the map can be used as Secret data.
// Loopback addresses in kubeconfigs are also replaced with the apiserver service name
func (k *KubernetesConfig) bytesFromDir(base string) (map[string][]byte, error) {
	fileBytes := map[string][]byte{}
	base = base + string(filepath.Separator)
	return fileBytes, filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// skip dynamiclistener secret cache, and anything that's not a plain file
		if d.Name() == "dynamic-cert.json" || !d.Type().IsRegular() {
			return nil
		}
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(d.Name(), ".kubeconfig") {
			b = bytes.ReplaceAll(b, []byte("[::1]"), []byte(k.Name+"-kube-apiserver"))
			b = bytes.ReplaceAll(b, []byte("127.0.0.1"), []byte(k.Name+"-kube-apiserver"))
		}
		path = strings.TrimPrefix(path, base)
		path = strings.ReplaceAll(path, string(filepath.Separator), "_")
		fileBytes[path] = b
		return nil
	})
}
