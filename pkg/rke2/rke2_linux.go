// +build linux

package rke2

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent/config"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/rancher/k3s/pkg/etcd"
	"github.com/rancher/rke2/pkg/cli/defaults"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/podexecutor"
	"github.com/urfave/cli"
)

func initExecutor(clx *cli.Context, cfg Config, dataDir string, disableETCD bool, isServer bool) (*podexecutor.StaticPodConfig, error) {
	// This flag will only be set on servers, on agents this is a no-op and the
	// resolver's default registry will get updated later when bootstrapping
	cfg.Images.SystemDefaultRegistry = clx.String("system-default-registry")
	resolver, err := images.NewResolver(cfg.Images)
	if err != nil {
		return nil, err
	}

	if err := defaults.Set(clx, dataDir); err != nil {
		return nil, err
	}

	agentManifestsDir := filepath.Join(dataDir, "agent", config.DefaultPodManifestPath)
	agentImagesDir := filepath.Join(dataDir, "agent", "images")

	managed.RegisterDriver(&etcd.ETCD{})

	if clx.IsSet("cloud-provider-config") || clx.IsSet("cloud-provider-name") {
		if clx.IsSet("node-external-ip") {
			return nil, errors.New("can't set node-external-ip while using cloud provider")
		}
		cmds.ServerConfig.DisableCCM = true
	}
	var cpConfig *podexecutor.CloudProviderConfig
	if cfg.CloudProviderConfig != "" && cfg.CloudProviderName == "" {
		return nil, fmt.Errorf("--cloud-provider-config requires --cloud-provider-name to be provided")
	}
	if cfg.CloudProviderName != "" {
		cpConfig = &podexecutor.CloudProviderConfig{
			Name: cfg.CloudProviderName,
			Path: cfg.CloudProviderConfig,
		}
	}

	if cfg.KubeletPath == "" {
		cfg.KubeletPath = "kubelet"
	}

	var controlPlaneResources podexecutor.ControlPlaneResources

	if cfg.ControlPlaneResourceRequests != "" {
		for _, r := range strings.Split(cfg.ControlPlaneResourceRequests, ",") {
			v := strings.Split(r, "=")
			if len(v) != 2 {
				logrus.Fatalf("incorrectly formatted control plane resource request specified: %s", r)
			}
			if strings.HasPrefix(v[0], KubeAPIServer) {
				resource := strings.TrimPrefix(v[0], KubeAPIServer+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.KubeAPIServerCPURequest = v[1]
				case "memory":
					controlPlaneResources.KubeAPIServerMemoryRequest = v[1]
				default:
					logrus.Fatalf("unrecognized resource request made: %s", r)
				}
			}
			if strings.HasPrefix(v[0], KubeScheduler) {
				resource := strings.TrimPrefix(v[0], KubeScheduler+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.KubeSchedulerCPURequest = v[1]
				case "memory":
					controlPlaneResources.KubeSchedulerMemoryRequest = v[1]
				default:
					logrus.Fatalf("unrecognized resource request made: %s", r)
				}
			}
			if strings.HasPrefix(v[0], KubeControllerManager) {
				resource := strings.TrimPrefix(v[0], KubeControllerManager+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.KubeControllerManagerCPURequest = v[1]
				case "memory":
					controlPlaneResources.KubeControllerManagerMemoryRequest = v[1]
				default:
					logrus.Fatalf("unrecognized resource request made: %s", r)
				}
			}
			if strings.HasPrefix(v[0], KubeProxy) {
				resource := strings.TrimPrefix(v[0], KubeProxy+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.KubeProxyCPURequest = v[1]
				case "memory":
					controlPlaneResources.KubeProxyMemoryRequest = v[1]
				default:
					logrus.Fatalf("unrecognized resource request made: %s", r)
				}
			}
			if strings.HasPrefix(v[0], Etcd) {
				resource := strings.TrimPrefix(v[0], Etcd+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.EtcdCPURequest = v[1]
				case "memory":
					controlPlaneResources.EtcdMemoryRequest = v[1]
				default:
					logrus.Fatalf("unrecognized resource request made: %s", r)
				}
			}
			if strings.HasPrefix(v[0], CloudControllerManager) {
				resource := strings.TrimPrefix(v[0], CloudControllerManager+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.CloudControllerManagerCPURequest = v[1]
				case "memory":
					controlPlaneResources.CloudControllerManagerMemoryRequest = v[1]
				default:
					logrus.Fatalf("unrecognized resource request made: %s", r)
				}
			}
		}
	}

	if cfg.ControlPlaneResourceLimits != "" {
		for _, r := range strings.Split(cfg.ControlPlaneResourceLimits, ",") {
			v := strings.Split(r, "=")
			if len(v) != 2 {
				logrus.Fatalf("incorrectly formatted control plane resource limit specified: %s", r)
			}
			if strings.HasPrefix(v[0], KubeAPIServer) {
				resource := strings.TrimPrefix(v[0], KubeAPIServer+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.KubeAPIServerCPULimit = v[1]
				case "memory":
					controlPlaneResources.KubeAPIServerMemoryLimit = v[1]
				default:
					logrus.Fatalf("unrecognized resource limit made: %s", r)
				}
			}
			if strings.HasPrefix(v[0], KubeScheduler) {
				resource := strings.TrimPrefix(v[0], KubeScheduler+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.KubeSchedulerCPULimit = v[1]
				case "memory":
					controlPlaneResources.KubeSchedulerMemoryLimit = v[1]
				default:
					logrus.Fatalf("unrecognized resource limit made: %s", r)
				}
			}
			if strings.HasPrefix(v[0], KubeControllerManager) {
				resource := strings.TrimPrefix(v[0], KubeControllerManager+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.KubeControllerManagerCPULimit = v[1]
				case "memory":
					controlPlaneResources.KubeControllerManagerMemoryLimit = v[1]
				default:
					logrus.Fatalf("unrecognized resource limit made: %s", r)
				}
			}
			if strings.HasPrefix(v[0], KubeProxy) {
				resource := strings.TrimPrefix(v[0], KubeProxy+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.KubeProxyCPULimit = v[1]
				case "memory":
					controlPlaneResources.KubeProxyMemoryLimit = v[1]
				default:
					logrus.Fatalf("unrecognized resource limit made: %s", r)
				}
			}
			if strings.HasPrefix(v[0], Etcd) {
				resource := strings.TrimPrefix(v[0], Etcd+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.EtcdCPULimit = v[1]
				case "memory":
					controlPlaneResources.EtcdMemoryLimit = v[1]
				default:
					logrus.Fatalf("unrecognized resource limit made: %s", r)
				}
			}
			if strings.HasPrefix(v[0], CloudControllerManager) {
				resource := strings.TrimPrefix(v[0], CloudControllerManager+"-")
				switch resource {
				case "cpu":
					controlPlaneResources.CloudControllerManagerCPULimit = v[1]
				case "memory":
					controlPlaneResources.CloudControllerManagerMemoryLimit = v[1]
				default:
					logrus.Fatalf("unrecognized resource limit made: %s", r)
				}
			}
		}
	}

	extraEnv := podexecutor.ControlPlaneEnv{
		KubeAPIServer:          cfg.ExtraEnv.KubeAPIServer.Value(),
		KubeScheduler:          cfg.ExtraEnv.KubeScheduler.Value(),
		KubeControllerManager:  cfg.ExtraEnv.KubeControllerManager.Value(),
		Etcd:                   cfg.ExtraEnv.Etcd.Value(),
		CloudControllerManager: cfg.ExtraEnv.CloudControllerManager.Value(),
	}

	extraBinds := podexecutor.ControlPlaneBinds{
		KubeAPIServer:          cfg.ExtraBinds.KubeAPIServer.Value(),
		KubeScheduler:          cfg.ExtraBinds.KubeScheduler.Value(),
		KubeControllerManager:  cfg.ExtraBinds.KubeControllerManager.Value(),
		Etcd:                   cfg.ExtraBinds.Etcd.Value(),
		CloudControllerManager: cfg.ExtraBinds.CloudControllerManager.Value(),
	}

	return &podexecutor.StaticPodConfig{
		Resolver:              resolver,
		ImagesDir:             agentImagesDir,
		ManifestsDir:          agentManifestsDir,
		CISMode:               isCISMode(clx),
		CloudProvider:         cpConfig,
		DataDir:               dataDir,
		AuditPolicyFile:       clx.String("audit-policy-file"),
		KubeletPath:           cfg.KubeletPath,
		DisableETCD:           disableETCD,
		IsServer:              isServer,
		ControlPlaneResources: controlPlaneResources,
		ControlPlaneEnv:       extraEnv,
		ControlPlaneBinds:     extraBinds,
	}, nil
}
