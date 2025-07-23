package cmds

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/k3s-io/k3s/pkg/version"
	rke2cli "github.com/rancher/rke2/pkg/cli"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/podtemplate"
	"github.com/urfave/cli/v2"
)

var (
	appName    = filepath.Base(os.Args[0])
	config     = rke2cli.Config{}
	commonFlag = []cli.Flag{
		&cli.StringFlag{
			Name:        images.KubeAPIServer,
			Usage:       "(image) Override image to use for kube-apiserver",
			EnvVars:     []string{"RKE2_KUBE_APISERVER_IMAGE"},
			Destination: &config.Images.KubeAPIServer,
		},
		&cli.StringFlag{
			Name:        images.KubeControllerManager,
			Usage:       "(image) Override image to use for kube-controller-manager",
			EnvVars:     []string{"RKE2_KUBE_CONTROLLER_MANAGER_IMAGE"},
			Destination: &config.Images.KubeControllerManager,
		},
		&cli.StringFlag{
			Name:        images.CloudControllerManager,
			Usage:       "(image) Override image to use for cloud-controller-manager",
			EnvVars:     []string{"RKE2_CLOUD_CONTROLLER_MANAGER_IMAGE"},
			Destination: &config.Images.CloudControllerManager,
		},
		&cli.StringFlag{
			Name:        images.KubeProxy,
			Usage:       "(image) Override image to use for kube-proxy",
			EnvVars:     []string{"RKE2_KUBE_PROXY_IMAGE"},
			Destination: &config.Images.KubeProxy,
		},
		&cli.StringFlag{
			Name:        images.KubeScheduler,
			Usage:       "(image) Override image to use for kube-scheduler",
			EnvVars:     []string{"RKE2_KUBE_SCHEDULER_IMAGE"},
			Destination: &config.Images.KubeScheduler,
		},
		&cli.StringFlag{
			Name:        images.Pause,
			Usage:       "(image) Override image to use for pause",
			EnvVars:     []string{"RKE2_PAUSE_IMAGE"},
			Destination: &config.Images.Pause,
		},
		&cli.StringFlag{
			Name:        images.Runtime,
			Usage:       "(image) Override image to use for runtime binaries (containerd, kubectl, crictl, etc)",
			EnvVars:     []string{"RKE2_RUNTIME_IMAGE"},
			Destination: &config.Images.Runtime,
		},
		&cli.StringFlag{
			Name:        images.ETCD,
			Usage:       "(image) Override image to use for etcd",
			EnvVars:     []string{"RKE2_ETCD_IMAGE"},
			Destination: &config.Images.ETCD,
		},
		&cli.StringFlag{
			Name:        "kubelet-path",
			Usage:       "(experimental/agent) Override kubelet binary path",
			EnvVars:     []string{"RKE2_KUBELET_PATH"},
			Destination: &config.KubeletPath,
		},
		&cli.StringFlag{
			Name:        "cloud-provider-name",
			Usage:       "(cloud provider) Cloud provider name",
			EnvVars:     []string{"RKE2_CLOUD_PROVIDER_NAME"},
			Destination: &config.CloudProviderName,
		},
		&cli.StringFlag{
			Name:        "cloud-provider-config",
			Usage:       "(cloud provider) Cloud provider configuration file path",
			EnvVars:     []string{"RKE2_CLOUD_PROVIDER_CONFIG"},
			Destination: &config.CloudProviderConfig,
		},
		&cli.BoolFlag{
			Name:        "node-name-from-cloud-provider-metadata",
			Usage:       "(cloud provider) Set node name from instance metadata service hostname",
			EnvVars:     []string{"RKE2_NODE_NAME_FROM_CLOUD_PROVIDER_METADATA"},
			Destination: &config.CloudProviderMetadataHostname,
		},
		&cli.StringFlag{
			Name:    "profile",
			Usage:   "(security) Validate system configuration against the selected benchmark (valid items: cis, etcd)",
			EnvVars: []string{"RKE2_CIS_PROFILE"},
		},
		&cli.StringFlag{
			Name:        "audit-policy-file",
			Usage:       "(security) Path to the file that defines the audit policy configuration",
			EnvVars:     []string{"RKE2_AUDIT_POLICY_FILE"},
			Destination: &config.AuditPolicyFile,
		},
		&cli.StringFlag{
			Name:        "pod-security-admission-config-file",
			Usage:       "(security) Path to the file that defines Pod Security Admission configuration",
			EnvVars:     []string{"RKE2_POD_SECURITY_ADMISSION_CONFIG_FILE"},
			Destination: &config.PodSecurityAdmissionConfigFile,
		},
		&cli.StringSliceFlag{
			Name:        "control-plane-resource-requests",
			Usage:       "(components) Control Plane resource requests",
			EnvVars:     []string{"RKE2_CONTROL_PLANE_RESOURCE_REQUESTS"},
			Destination: &config.ControlPlaneResourceRequests,
		},
		&cli.StringSliceFlag{
			Name:        "control-plane-resource-limits",
			Usage:       "(components) Control Plane resource limits",
			EnvVars:     []string{"RKE2_CONTROL_PLANE_RESOURCE_LIMITS"},
			Destination: &config.ControlPlaneResourceLimits,
		},
		&cli.StringSliceFlag{
			Name:        "control-plane-probe-configuration",
			Usage:       "(components) Control Plane Probe configuration",
			EnvVars:     []string{"RKE2_CONTROL_PLANE_PROBE_CONFIGURATION"},
			Destination: &config.ControlPlaneProbeConf,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.KubeAPIServer + "-extra-mount",
			Usage:       "(components) " + podtemplate.KubeAPIServer + " extra volume mounts",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.KubeAPIServer, "-", "_")) + "_EXTRA_MOUNT"},
			Destination: &config.ExtraMounts.KubeAPIServer,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.KubeScheduler + "-extra-mount",
			Usage:       "(components) " + podtemplate.KubeScheduler + " extra volume mounts",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.KubeScheduler, "-", "_")) + "_EXTRA_MOUNT"},
			Destination: &config.ExtraMounts.KubeScheduler,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.KubeControllerManager + "-extra-mount",
			Usage:       "(components) " + podtemplate.KubeControllerManager + " extra volume mounts",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.KubeControllerManager, "-", "_")) + "_EXTRA_MOUNT"},
			Destination: &config.ExtraMounts.KubeControllerManager,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.KubeProxy + "-extra-mount",
			Usage:       "(components) " + podtemplate.KubeProxy + " extra volume mounts",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.KubeProxy, "-", "_")) + "_EXTRA_MOUNT"},
			Destination: &config.ExtraMounts.KubeProxy,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.Etcd + "-extra-mount",
			Usage:       "(components) " + podtemplate.Etcd + " extra volume mounts",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.Etcd, "-", "_")) + "_EXTRA_MOUNT"},
			Destination: &config.ExtraMounts.Etcd,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.CloudControllerManager + "-extra-mount",
			Usage:       "(components) " + podtemplate.CloudControllerManager + " extra volume mounts",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.CloudControllerManager, "-", "_")) + "_EXTRA_MOUNT"},
			Destination: &config.ExtraMounts.CloudControllerManager,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.KubeAPIServer + "-extra-env",
			Usage:       "(components) " + podtemplate.KubeAPIServer + " extra environment variables",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.KubeAPIServer, "-", "_")) + "_EXTRA_ENV"},
			Destination: &config.ExtraEnv.KubeAPIServer,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.KubeScheduler + "-extra-env",
			Usage:       "(components) " + podtemplate.KubeScheduler + " extra environment variables",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.KubeScheduler, "-", "_")) + "_EXTRA_ENV"},
			Destination: &config.ExtraEnv.KubeScheduler,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.KubeControllerManager + "-extra-env",
			Usage:       "(components) " + podtemplate.KubeControllerManager + " extra environment variables",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.KubeControllerManager, "-", "_")) + "_EXTRA_ENV"},
			Destination: &config.ExtraEnv.KubeControllerManager,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.KubeProxy + "-extra-env",
			Usage:       "(components) " + podtemplate.KubeProxy + " extra environment variables",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.KubeProxy, "-", "_")) + "_EXTRA_ENV"},
			Destination: &config.ExtraEnv.KubeProxy,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.Etcd + "-extra-env",
			Usage:       "(components) " + podtemplate.Etcd + " extra environment variables",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.Etcd, "-", "_")) + "_EXTRA_ENV"},
			Destination: &config.ExtraEnv.Etcd,
		},
		&cli.StringSliceFlag{
			Name:        podtemplate.CloudControllerManager + "-extra-env",
			Usage:       "(components) " + podtemplate.CloudControllerManager + " extra environment variables",
			EnvVars:     []string{"RKE2_" + strings.ToUpper(strings.ReplaceAll(podtemplate.CloudControllerManager, "-", "_")) + "_EXTRA_ENV"},
			Destination: &config.ExtraEnv.CloudControllerManager,
		},
	}
)

type CLIRole int64

const (
	Agent CLIRole = iota
	Server
)

func init() {
	// hack - force "file,dns" lookup order if go dns is used
	if os.Getenv("RES_OPTIONS") == "" {
		os.Setenv("RES_OPTIONS", " ")
	}
}

func validateCloudProviderName(clx *cli.Context, role CLIRole) {
	cloudProvider := clx.String("cloud-provider-name")
	cloudProviderDisables := map[string][]string{
		"rancher-vsphere": {"rancher-vsphere-cpi", "rancher-vsphere-csi"},
		"harvester":       {"harvester-cloud-provider", "harvester-csi-driver"},
	}

	for providerName, disables := range cloudProviderDisables {
		if providerName == cloudProvider {
			clx.Set("cloud-provider-name", "external")
			if role == Server {
				clx.Set("disable-cloud-controller", "true")
			}
		} else {
			if role == Server {
				for _, disable := range disables {
					clx.Set("disable", disable)
				}
			}
		}
	}
}

func NewApp() *cli.App {
	app := cli.NewApp()
	app.Name = appName
	app.EnableBashCompletion = true
	app.DisableSliceFlagSeparator = true
	app.Usage = "Rancher Kubernetes Engine 2"
	app.Version = fmt.Sprintf("%s (%s)", version.Version, version.GitCommit)
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s version %s\n", app.Name, app.Version)
		fmt.Printf("go version %s\n", runtime.Version())
	}

	return app
}
