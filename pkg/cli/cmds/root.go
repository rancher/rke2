package cmds

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/urfave/cli"
)

var (
	appName    = filepath.Base(os.Args[0])
	commonFlag = []cli.Flag{
		&cli.StringFlag{
			Name:        images.KubeAPIServer,
			Usage:       "(image) Override image to use for kube-apiserver",
			EnvVar:      "RKE2_KUBE_APISERVER_IMAGE",
			Destination: &config.Images.KubeAPIServer,
		},
		&cli.StringFlag{
			Name:        images.KubeControllerManager,
			Usage:       "(image) Override image to use for kube-controller-manager",
			EnvVar:      "RKE2_KUBE_CONTROLLER_MANAGER_IMAGE",
			Destination: &config.Images.KubeControllerManager,
		},
		&cli.StringFlag{
			Name:        images.CloudControllerManager,
			Usage:       "(image) Override image to use for cloud-controller-manager",
			EnvVar:      "RKE2_CLOUD_CONTROLLER_MANAGER_IMAGE",
			Destination: &config.Images.CloudControllerManager,
		},
		&cli.StringFlag{
			Name:        images.KubeProxy,
			Usage:       "(image) Override image to use for kube-proxy",
			EnvVar:      "RKE2_KUBE_PROXY_IMAGE",
			Destination: &config.Images.KubeProxy,
		},
		&cli.StringFlag{
			Name:        images.KubeScheduler,
			Usage:       "(image) Override image to use for kube-scheduler",
			EnvVar:      "RKE2_KUBE_SCHEDULER_IMAGE",
			Destination: &config.Images.KubeScheduler,
		},
		&cli.StringFlag{
			Name:        images.Pause,
			Usage:       "(image) Override image to use for pause",
			EnvVar:      "RKE2_PAUSE_IMAGE",
			Destination: &config.Images.Pause,
		},
		&cli.StringFlag{
			Name:        images.Runtime,
			Usage:       "(image) Override image to use for runtime binaries (containerd, kubectl, crictl, etc)",
			EnvVar:      "RKE2_RUNTIME_IMAGE",
			Destination: &config.Images.Runtime,
		},
		&cli.StringFlag{
			Name:        images.ETCD,
			Usage:       "(image) Override image to use for etcd",
			EnvVar:      "RKE2_ETCD_IMAGE",
			Destination: &config.Images.ETCD,
		},
		&cli.StringFlag{
			Name:        "kubelet-path",
			Usage:       "(experimental/agent) Override kubelet binary path",
			EnvVar:      "RKE2_KUBELET_PATH",
			Destination: &config.KubeletPath,
		},
		&cli.StringFlag{
			Name:        "cloud-provider-name",
			Usage:       "(cloud provider) Cloud provider name",
			EnvVar:      "RKE2_CLOUD_PROVIDER_NAME",
			Destination: &config.CloudProviderName,
		},
		&cli.StringFlag{
			Name:        "cloud-provider-config",
			Usage:       "(cloud provider) Cloud provider configuration file path",
			EnvVar:      "RKE2_CLOUD_PROVIDER_CONFIG",
			Destination: &config.CloudProviderConfig,
		},
		&cli.BoolFlag{
			Name:        "node-name-from-cloud-provider-metadata",
			Usage:       "(cloud provider) Set node name from instance metadata service hostname",
			EnvVar:      "RKE2_NODE_NAME_FROM_CLOUD_PROVIDER_METADATA",
			Destination: &config.CloudProviderMetadataHostname,
		},
		&cli.StringFlag{
			Name:   "profile",
			Usage:  "(security) Validate system configuration against the selected benchmark (valid items: cis)",
			EnvVar: "RKE2_CIS_PROFILE",
		},
		&cli.StringFlag{
			Name:        "audit-policy-file",
			Usage:       "(security) Path to the file that defines the audit policy configuration",
			EnvVar:      "RKE2_AUDIT_POLICY_FILE",
			Destination: &config.AuditPolicyFile,
		},
		&cli.StringFlag{
			Name:        "pod-security-admission-config-file",
			Usage:       "(security) Path to the file that defines Pod Security Admission configuration",
			EnvVar:      "RKE2_POD_SECURITY_ADMISSION_CONFIG_FILE",
			Destination: &config.PodSecurityAdmissionConfigFile,
		},
		&cli.StringSliceFlag{
			Name:   "control-plane-resource-requests",
			Usage:  "(components) Control Plane resource requests",
			EnvVar: "RKE2_CONTROL_PLANE_RESOURCE_REQUESTS",
			Value:  &config.ControlPlaneResourceRequests,
		},
		&cli.StringSliceFlag{
			Name:   "control-plane-resource-limits",
			Usage:  "(components) Control Plane resource limits",
			EnvVar: "RKE2_CONTROL_PLANE_RESOURCE_LIMITS",
			Value:  &config.ControlPlaneResourceLimits,
		},
		&cli.StringSliceFlag{
			Name:   "control-plane-probe-configuration",
			Usage:  "(components) Control Plane Probe configuration",
			EnvVar: "RKE2_CONTROL_PLANE_PROBE_CONFIGURATION",
			Value:  &config.ControlPlaneProbeConf,
		},
		&cli.StringSliceFlag{
			Name:   rke2.KubeAPIServer + "-extra-mount",
			Usage:  "(components) " + rke2.KubeAPIServer + " extra volume mounts",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.KubeAPIServer, "-", "_")) + "_EXTRA_MOUNT",
			Value:  &config.ExtraMounts.KubeAPIServer,
		},
		&cli.StringSliceFlag{
			Name:   rke2.KubeScheduler + "-extra-mount",
			Usage:  "(components) " + rke2.KubeScheduler + " extra volume mounts",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.KubeScheduler, "-", "_")) + "_EXTRA_MOUNT",
			Value:  &config.ExtraMounts.KubeScheduler,
		},
		&cli.StringSliceFlag{
			Name:   rke2.KubeControllerManager + "-extra-mount",
			Usage:  "(components) " + rke2.KubeControllerManager + " extra volume mounts",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.KubeControllerManager, "-", "_")) + "_EXTRA_MOUNT",
			Value:  &config.ExtraMounts.KubeControllerManager,
		},
		&cli.StringSliceFlag{
			Name:   rke2.KubeProxy + "-extra-mount",
			Usage:  "(components) " + rke2.KubeProxy + " extra volume mounts",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.KubeProxy, "-", "_")) + "_EXTRA_MOUNT",
			Value:  &config.ExtraMounts.KubeProxy,
		},
		&cli.StringSliceFlag{
			Name:   rke2.Etcd + "-extra-mount",
			Usage:  "(components) " + rke2.Etcd + " extra volume mounts",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.Etcd, "-", "_")) + "_EXTRA_MOUNT",
			Value:  &config.ExtraMounts.Etcd,
		},
		&cli.StringSliceFlag{
			Name:   rke2.CloudControllerManager + "-extra-mount",
			Usage:  "(components) " + rke2.CloudControllerManager + " extra volume mounts",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.CloudControllerManager, "-", "_")) + "_EXTRA_MOUNT",
			Value:  &config.ExtraMounts.CloudControllerManager,
		},
		&cli.StringSliceFlag{
			Name:   rke2.KubeAPIServer + "-extra-env",
			Usage:  "(components) " + rke2.KubeAPIServer + " extra environment variables",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.KubeAPIServer, "-", "_")) + "_EXTRA_ENV",
			Value:  &config.ExtraEnv.KubeAPIServer,
		},
		&cli.StringSliceFlag{
			Name:   rke2.KubeScheduler + "-extra-env",
			Usage:  "(components) " + rke2.KubeScheduler + " extra environment variables",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.KubeScheduler, "-", "_")) + "_EXTRA_ENV",
			Value:  &config.ExtraEnv.KubeScheduler,
		},
		&cli.StringSliceFlag{
			Name:   rke2.KubeControllerManager + "-extra-env",
			Usage:  "(components) " + rke2.KubeControllerManager + " extra environment variables",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.KubeControllerManager, "-", "_")) + "_EXTRA_ENV",
			Value:  &config.ExtraEnv.KubeControllerManager,
		},
		&cli.StringSliceFlag{
			Name:   rke2.KubeProxy + "-extra-env",
			Usage:  "(components) " + rke2.KubeProxy + " extra environment variables",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.KubeProxy, "-", "_")) + "_EXTRA_ENV",
			Value:  &config.ExtraEnv.KubeProxy,
		},
		&cli.StringSliceFlag{
			Name:   rke2.Etcd + "-extra-env",
			Usage:  "(components) " + rke2.Etcd + " extra environment variables",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.Etcd, "-", "_")) + "_EXTRA_ENV",
			Value:  &config.ExtraEnv.Etcd,
		},
		&cli.StringSliceFlag{
			Name:   rke2.CloudControllerManager + "-extra-env",
			Usage:  "(components) " + rke2.CloudControllerManager + " extra environment variables",
			EnvVar: "RKE2_" + strings.ToUpper(strings.ReplaceAll(rke2.CloudControllerManager, "-", "_")) + "_EXTRA_ENV",
			Value:  &config.ExtraEnv.CloudControllerManager,
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
	app.Usage = "Rancher Kubernetes Engine 2"
	app.Version = fmt.Sprintf("%s (%s)", version.Version, version.GitCommit)
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s version %s\n", app.Name, app.Version)
		fmt.Printf("go version %s\n", runtime.Version())
	}

	return app
}
