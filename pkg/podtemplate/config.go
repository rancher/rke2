package podtemplate

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	rke2cli "github.com/rancher/rke2/pkg/cli"
	"github.com/rancher/rke2/pkg/images"
	"github.com/urfave/cli/v2"
)

func NewConfigFromCLI(dataDir string, cfg rke2cli.Config) (*Config, error) {
	resolver, err := images.NewResolver(cfg.Images)
	if err != nil {
		return nil, err
	}

	controlPlaneResources, err := parseControlPlaneResources(&cfg.ControlPlaneResourceRequests, &cfg.ControlPlaneResourceLimits)
	if err != nil {
		return nil, err
	}

	controlPlaneProbeConfs, err := parseControlPlaneProbeConfs(&cfg.ControlPlaneProbeConf)
	if err != nil {
		return nil, err
	}

	env, err := parseControlPlaneEnv(cfg.ExtraEnv)
	if err != nil {
		return nil, err
	}

	mounts, err := parseControlPlaneMounts(cfg.ExtraMounts)
	if err != nil {
		return nil, err
	}

	return &Config{
		Resolver:  resolver,
		ImagesDir: filepath.Join(dataDir, "agent", "images"),
		DataDir:   dataDir,
		Resources: controlPlaneResources,
		Probes:    controlPlaneProbeConfs,
		Env:       env,
		Mounts:    mounts,
	}, nil
}

func parseControlPlaneResources(requests, limits *cli.StringSlice) (*ControlPlaneResources, error) {
	var controlPlaneResources ControlPlaneResources
	// resources is a map of the component (kube-apiserver, kube-controller-manager, etc.) to a map[string]*string,
	// where the key of the downstream map is the `cpu-request`, `cpu-limit`, `memory-request`, or `memory-limit` and
	// the value corresponds to a pointer to the component resources array
	resources := map[string]map[string]*string{
		KubeAPIServer: {
			CPURequest:    &controlPlaneResources.KubeAPIServerCPURequest,
			CPULimit:      &controlPlaneResources.KubeAPIServerCPULimit,
			MemoryRequest: &controlPlaneResources.KubeAPIServerMemoryRequest,
			MemoryLimit:   &controlPlaneResources.KubeAPIServerMemoryLimit,
		},
		KubeScheduler: {
			CPURequest:    &controlPlaneResources.KubeSchedulerCPURequest,
			CPULimit:      &controlPlaneResources.KubeSchedulerCPULimit,
			MemoryRequest: &controlPlaneResources.KubeSchedulerMemoryRequest,
			MemoryLimit:   &controlPlaneResources.KubeSchedulerMemoryLimit,
		},
		KubeControllerManager: {
			CPURequest:    &controlPlaneResources.KubeControllerManagerCPURequest,
			CPULimit:      &controlPlaneResources.KubeControllerManagerCPULimit,
			MemoryRequest: &controlPlaneResources.KubeControllerManagerMemoryRequest,
			MemoryLimit:   &controlPlaneResources.KubeControllerManagerMemoryLimit,
		},
		KubeProxy: {
			CPURequest:    &controlPlaneResources.KubeProxyCPURequest,
			CPULimit:      &controlPlaneResources.KubeProxyCPULimit,
			MemoryRequest: &controlPlaneResources.KubeProxyMemoryRequest,
			MemoryLimit:   &controlPlaneResources.KubeProxyMemoryLimit,
		},
		Etcd: {
			CPURequest:    &controlPlaneResources.EtcdCPURequest,
			CPULimit:      &controlPlaneResources.EtcdCPULimit,
			MemoryRequest: &controlPlaneResources.EtcdMemoryRequest,
			MemoryLimit:   &controlPlaneResources.EtcdMemoryLimit,
		},
		CloudControllerManager: {
			CPURequest:    &controlPlaneResources.CloudControllerManagerCPURequest,
			CPULimit:      &controlPlaneResources.CloudControllerManagerCPULimit,
			MemoryRequest: &controlPlaneResources.CloudControllerManagerMemoryRequest,
			MemoryLimit:   &controlPlaneResources.CloudControllerManagerMemoryLimit,
		},
	}

	// defaultResources contains a map of default resources for each component, used if not explicitly configured.
	defaultResources := map[string]map[string]string{
		KubeAPIServer: {
			CPURequest:    "250m",
			MemoryRequest: "1024Mi",
		},
		KubeScheduler: {
			CPURequest:    "100m",
			MemoryRequest: "128Mi",
		},
		KubeControllerManager: {
			CPURequest:    "200m",
			MemoryRequest: "256Mi",
		},
		KubeProxy: {
			CPURequest:    "250m",
			MemoryRequest: "128Mi",
		},
		Etcd: {
			CPURequest:    "200m",
			MemoryRequest: "512Mi",
		},
		CloudControllerManager: {
			CPURequest:    "100m",
			MemoryRequest: "128Mi",
		},
	}

	parsedRequestsLimits := make(map[string]string)

	for _, requests := range requests.Value() {
		for _, rawRequest := range strings.Split(requests, ",") {
			v := strings.SplitN(rawRequest, "=", 2)
			if len(v) != 2 {
				return nil, fmt.Errorf("incorrectly formatted control plane resource request specified: %s", rawRequest)
			}
			parsedRequestsLimits[v[0]+"-request"] = v[1]
		}
	}

	for _, limits := range limits.Value() {
		for _, rawLimit := range strings.Split(limits, ",") {
			v := strings.SplitN(rawLimit, "=", 2)
			if len(v) != 2 {
				return nil, fmt.Errorf("incorrectly formatted control plane resource limit specified: %s", rawLimit)
			}
			parsedRequestsLimits[v[0]+"-limit"] = v[1]
		}
	}

	for component, request := range resources {
		for resource, target := range request {
			k := component + "-" + resource
			if val, ok := parsedRequestsLimits[k]; ok {
				*target = val
			} else if val, ok := defaultResources[component][resource]; ok {
				*target = val
			}
		}
	}

	return &controlPlaneResources, nil
}

func parseControlPlaneProbeConfs(probeConfs *cli.StringSlice) (*ControlPlaneProbeConfs, error) {
	var controlPlaneProbes ControlPlaneProbeConfs
	// probes is a map of the component (kube-apiserver, kube-controller-manager, etc.) probe type, and setting, where
	// the value corresponds to a pointer to the component probes array.
	probes := map[string]map[string]map[string]*int32{
		KubeAPIServer: {
			Liveness: {
				InitialDelaySeconds: &controlPlaneProbes.KubeAPIServer.Liveness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeAPIServer.Liveness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeAPIServer.Liveness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeAPIServer.Liveness.PeriodSeconds,
			},
			Readiness: {
				InitialDelaySeconds: &controlPlaneProbes.KubeAPIServer.Readiness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeAPIServer.Readiness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeAPIServer.Readiness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeAPIServer.Readiness.PeriodSeconds,
			},
			Startup: {
				InitialDelaySeconds: &controlPlaneProbes.KubeAPIServer.Startup.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeAPIServer.Startup.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeAPIServer.Startup.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeAPIServer.Startup.PeriodSeconds,
			},
		},
		KubeScheduler: {
			Liveness: {
				InitialDelaySeconds: &controlPlaneProbes.KubeScheduler.Liveness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeScheduler.Liveness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeScheduler.Liveness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeScheduler.Liveness.PeriodSeconds,
			},
			Readiness: {
				InitialDelaySeconds: &controlPlaneProbes.KubeScheduler.Readiness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeScheduler.Readiness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeScheduler.Readiness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeScheduler.Readiness.PeriodSeconds,
			},
			Startup: {
				InitialDelaySeconds: &controlPlaneProbes.KubeScheduler.Startup.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeScheduler.Startup.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeScheduler.Startup.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeScheduler.Startup.PeriodSeconds,
			},
		},
		KubeControllerManager: {
			Liveness: {
				InitialDelaySeconds: &controlPlaneProbes.KubeControllerManager.Liveness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeControllerManager.Liveness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeControllerManager.Liveness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeControllerManager.Liveness.PeriodSeconds,
			},
			Readiness: {
				InitialDelaySeconds: &controlPlaneProbes.KubeControllerManager.Readiness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeControllerManager.Readiness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeControllerManager.Readiness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeControllerManager.Readiness.PeriodSeconds,
			},
			Startup: {
				InitialDelaySeconds: &controlPlaneProbes.KubeControllerManager.Startup.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeControllerManager.Startup.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeControllerManager.Startup.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeControllerManager.Startup.PeriodSeconds,
			},
		},
		KubeProxy: {
			Liveness: {
				InitialDelaySeconds: &controlPlaneProbes.KubeProxy.Liveness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeProxy.Liveness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeProxy.Liveness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeProxy.Liveness.PeriodSeconds,
			},
			Readiness: {
				InitialDelaySeconds: &controlPlaneProbes.KubeProxy.Readiness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeProxy.Readiness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeProxy.Readiness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeProxy.Readiness.PeriodSeconds,
			},
			Startup: {
				InitialDelaySeconds: &controlPlaneProbes.KubeProxy.Startup.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.KubeProxy.Startup.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.KubeProxy.Startup.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.KubeProxy.Startup.PeriodSeconds,
			},
		},
		Etcd: {
			Liveness: {
				InitialDelaySeconds: &controlPlaneProbes.Etcd.Liveness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.Etcd.Liveness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.Etcd.Liveness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.Etcd.Liveness.PeriodSeconds,
			},
			Readiness: {
				InitialDelaySeconds: &controlPlaneProbes.Etcd.Readiness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.Etcd.Readiness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.Etcd.Readiness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.Etcd.Readiness.PeriodSeconds,
			},
			Startup: {
				InitialDelaySeconds: &controlPlaneProbes.Etcd.Startup.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.Etcd.Startup.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.Etcd.Startup.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.Etcd.Startup.PeriodSeconds,
			},
		},
		CloudControllerManager: {
			Liveness: {
				InitialDelaySeconds: &controlPlaneProbes.CloudControllerManager.Liveness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.CloudControllerManager.Liveness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.CloudControllerManager.Liveness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.CloudControllerManager.Liveness.PeriodSeconds,
			},
			Readiness: {
				InitialDelaySeconds: &controlPlaneProbes.CloudControllerManager.Readiness.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.CloudControllerManager.Readiness.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.CloudControllerManager.Readiness.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.CloudControllerManager.Readiness.PeriodSeconds,
			},
			Startup: {
				InitialDelaySeconds: &controlPlaneProbes.CloudControllerManager.Startup.InitialDelaySeconds,
				TimeoutSeconds:      &controlPlaneProbes.CloudControllerManager.Startup.TimeoutSeconds,
				FailureThreshold:    &controlPlaneProbes.CloudControllerManager.Startup.FailureThreshold,
				PeriodSeconds:       &controlPlaneProbes.CloudControllerManager.Startup.PeriodSeconds,
			},
		},
	}

	// defaultProbeConf contains a map of default probe settings for each type, used if not explicitly configured.
	defaultProbeConf := map[string]map[string]int32{
		// https://github.com/kubernetes/kubernetes/blob/v1.24.0/cmd/kubeadm/app/util/staticpod/utils.go#L246
		Liveness: {
			InitialDelaySeconds: 10,
			TimeoutSeconds:      15,
			FailureThreshold:    8,
			PeriodSeconds:       10,
		},
		// https://github.com/kubernetes/kubernetes/blob/v1.24.0/cmd/kubeadm/app/util/staticpod/utils.go#L252
		Readiness: {
			InitialDelaySeconds: 0,
			TimeoutSeconds:      15,
			FailureThreshold:    3,
			PeriodSeconds:       1,
		},
		// https://github.com/kubernetes/kubernetes/blob/v1.24.0/cmd/kubeadm/app/util/staticpod/utils.go#L259
		Startup: {
			InitialDelaySeconds: 10,
			TimeoutSeconds:      15,
			FailureThreshold:    24,
			PeriodSeconds:       10,
		},
	}

	parsedProbeConf := make(map[string]int32)

	for _, conf := range probeConfs.Value() {
		for _, rawConf := range strings.Split(conf, ",") {
			v := strings.SplitN(rawConf, "=", 2)
			if len(v) != 2 {
				return nil, fmt.Errorf("incorrectly formatted control probe config specified: %s", rawConf)
			}
			val, err := strconv.ParseInt(v[1], 10, 32)
			if err != nil || val < 0 {
				return nil, fmt.Errorf("invalid control plane probe config value specified: %s", rawConf)
			}
			parsedProbeConf[v[0]] = int32(val)
		}
	}

	for component, probe := range probes {
		for probeName, conf := range probe {
			for threshold, target := range conf {
				k := component + "-" + probeName + "-" + threshold
				if val, ok := parsedProbeConf[k]; ok {
					*target = val
				} else if val, ok := defaultProbeConf[probeName][threshold]; ok {
					*target = val
				}
			}
		}
	}

	return &controlPlaneProbes, nil
}

func parseControlPlaneEnv(extraEnv rke2cli.ExtraEnv) (*ControlPlaneEnv, error) {
	return &ControlPlaneEnv{
		KubeAPIServer:          extraEnv.KubeAPIServer.Value(),
		KubeScheduler:          extraEnv.KubeScheduler.Value(),
		KubeControllerManager:  extraEnv.KubeControllerManager.Value(),
		KubeProxy:              extraEnv.KubeProxy.Value(),
		Etcd:                   extraEnv.Etcd.Value(),
		CloudControllerManager: extraEnv.CloudControllerManager.Value(),
	}, nil
}

func parseControlPlaneMounts(extraMounts rke2cli.ExtraMounts) (*ControlPlaneMounts, error) {
	return &ControlPlaneMounts{
		KubeAPIServer:          extraMounts.KubeAPIServer.Value(),
		KubeScheduler:          extraMounts.KubeScheduler.Value(),
		KubeControllerManager:  extraMounts.KubeControllerManager.Value(),
		KubeProxy:              extraMounts.KubeProxy.Value(),
		Etcd:                   extraMounts.Etcd.Value(),
		CloudControllerManager: extraMounts.CloudControllerManager.Value(),
	}, nil
}
