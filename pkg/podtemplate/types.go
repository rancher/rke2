package podtemplate

import (
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/rancher/rke2/pkg/images"
	v1 "k8s.io/api/core/v1"
)

const (
	KubeAPIServer          = "kube-apiserver"
	KubeScheduler          = "kube-scheduler"
	KubeControllerManager  = "kube-controller-manager"
	KubeProxy              = "kube-proxy"
	Etcd                   = "etcd"
	CloudControllerManager = "cloud-controller-manager"

	CPURequest    = "cpu-request"
	CPULimit      = "cpu-limit"
	MemoryRequest = "memory-request"
	MemoryLimit   = "memory-limit"

	Readiness = "readiness"
	Liveness  = "liveness"
	Startup   = "startup"

	InitialDelaySeconds = "initial-delay-seconds"
	TimeoutSeconds      = "timeout-seconds"
	FailureThreshold    = "failure-threshold"
	PeriodSeconds       = "period-seconds"
)

var (
	SSLDirs = []string{
		"/etc/ssl/certs",
		"/etc/pki/tls/certs",
		"/etc/ca-certificates",
		"/usr/local/share/ca-certificates",
		"/usr/share/ca-certificates",
	}
	DefaultAuditPolicyFile = "/etc/rancher/rke2/audit-policy.yaml"
)

type Config struct {
	ImagesDir string
	DataDir   string
	Resolver  *images.Resolver
	Env       *ControlPlaneEnv
	Mounts    *ControlPlaneMounts
	Probes    *ControlPlaneProbeConfs
	Resources *ControlPlaneResources
}

type Spec struct {
	Command         string
	Args            []string
	Image           name.Reference
	Dirs            []string
	Files           []string
	Sockets         []string
	ExcludeFiles    []string
	StartupExec     []string
	StartupPort     int32
	StartupScheme   string
	StartupPath     string
	HealthExec      []string
	HealthPort      int32
	HealthScheme    string
	HealthPath      string
	ReadyExec       []string
	ReadyPort       int32
	ReadyScheme     string
	ReadyPath       string
	CPURequest      string
	CPULimit        string
	MemoryRequest   string
	MemoryLimit     string
	ExtraMounts     []string
	ExtraEnv        []string
	ProbeConfs      ProbeConfs
	SecurityContext *v1.PodSecurityContext
	Ports           []v1.ContainerPort
	Annotations     map[string]string
	Privileged      bool
	HostNetwork     bool
}

type ControlPlaneResources struct {
	KubeAPIServerCPURequest             string
	KubeAPIServerCPULimit               string
	KubeAPIServerMemoryRequest          string
	KubeAPIServerMemoryLimit            string
	KubeSchedulerCPURequest             string
	KubeSchedulerCPULimit               string
	KubeSchedulerMemoryRequest          string
	KubeSchedulerMemoryLimit            string
	KubeControllerManagerCPURequest     string
	KubeControllerManagerCPULimit       string
	KubeControllerManagerMemoryRequest  string
	KubeControllerManagerMemoryLimit    string
	KubeProxyCPURequest                 string
	KubeProxyCPULimit                   string
	KubeProxyMemoryRequest              string
	KubeProxyMemoryLimit                string
	EtcdCPURequest                      string
	EtcdCPULimit                        string
	EtcdMemoryRequest                   string
	EtcdMemoryLimit                     string
	CloudControllerManagerCPURequest    string
	CloudControllerManagerCPULimit      string
	CloudControllerManagerMemoryRequest string
	CloudControllerManagerMemoryLimit   string
}

type ControlPlaneEnv struct {
	KubeAPIServer          []string
	KubeScheduler          []string
	KubeControllerManager  []string
	KubeProxy              []string
	Etcd                   []string
	CloudControllerManager []string
}

type ControlPlaneMounts struct {
	KubeAPIServer          []string
	KubeScheduler          []string
	KubeControllerManager  []string
	KubeProxy              []string
	Etcd                   []string
	CloudControllerManager []string
}

type ControlPlaneProbeConfs struct {
	KubeAPIServer          ProbeConfs
	KubeScheduler          ProbeConfs
	KubeControllerManager  ProbeConfs
	KubeProxy              ProbeConfs
	Etcd                   ProbeConfs
	CloudControllerManager ProbeConfs
}

type ProbeConf struct {
	InitialDelaySeconds int32
	TimeoutSeconds      int32
	FailureThreshold    int32
	PeriodSeconds       int32
}

type ProbeConfs struct {
	Liveness  ProbeConf
	Readiness ProbeConf
	Startup   ProbeConf
}
