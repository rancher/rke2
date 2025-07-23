package cli

import (
	"github.com/rancher/rke2/pkg/images"
	urfave "github.com/urfave/cli/v2"
)

var (
	DisableItems = []string{"rke2-coredns", "rke2-metrics-server", "rke2-snapshot-controller", "rke2-snapshot-controller-crd", "rke2-snapshot-validation-webhook"}
	CNIItems     = []string{"calico", "canal", "cilium", "flannel"}
	IngressItems = []string{"ingress-nginx", "traefik"}
)

type Config struct {
	AuditPolicyFile                string
	PodSecurityAdmissionConfigFile string
	CloudProviderConfig            string
	CloudProviderName              string
	CloudProviderMetadataHostname  bool
	Images                         images.ImageOverrideConfig
	KubeletPath                    string
	ControlPlaneResourceRequests   urfave.StringSlice
	ControlPlaneResourceLimits     urfave.StringSlice
	ControlPlaneProbeConf          urfave.StringSlice
	CNI                            urfave.StringSlice
	IngressController              urfave.StringSlice
	ExtraMounts                    ExtraMounts
	ExtraEnv                       ExtraEnv
}

type ExtraMounts struct {
	KubeAPIServer          urfave.StringSlice
	KubeScheduler          urfave.StringSlice
	KubeControllerManager  urfave.StringSlice
	KubeProxy              urfave.StringSlice
	Etcd                   urfave.StringSlice
	CloudControllerManager urfave.StringSlice
}

type ExtraEnv struct {
	KubeAPIServer          urfave.StringSlice
	KubeScheduler          urfave.StringSlice
	KubeControllerManager  urfave.StringSlice
	KubeProxy              urfave.StringSlice
	Etcd                   urfave.StringSlice
	CloudControllerManager urfave.StringSlice
}
