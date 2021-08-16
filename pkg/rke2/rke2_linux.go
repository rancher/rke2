// +build linux

package rke2

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent/config"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/rancher/k3s/pkg/etcd"
	"github.com/rancher/rke2/pkg/cli/defaults"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/podexecutor"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	CPURequest    = "cpu-request"
	CPULimit      = "cpu-limit"
	MemoryRequest = "memory-request"
	MemoryLimit   = "memory-limit"
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
		if clx.String("node-name") == "" && cfg.CloudProviderName == "aws" {
			fqdn, err := hostnameFQDN()
			if err != nil {
				return nil, err
			}
			if err := clx.Set("node-name", fqdn); err != nil {
				return nil, err
			}
		}
	}

	if cfg.KubeletPath == "" {
		cfg.KubeletPath = "kubelet"
	}

	var controlPlaneResources podexecutor.ControlPlaneResources
	// resources is a map of the component (kube-apiserver, kube-controller-manager, etc.) to a map[string]*string,
	// where the key of the downstream map is the `cpu-request`, `cpu-limit`, `memory-request`, or `memory-limit` and
	//the value corresponds to a pointer to the component resources array
	var resources = map[string]map[string]*string{
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

	var parsedRequestsLimits = make(map[string]string)

	if cfg.ControlPlaneResourceRequests != "" {
		for _, rawRequest := range strings.Split(cfg.ControlPlaneResourceRequests, ",") {
			v := strings.SplitN(rawRequest, "=", 2)
			if len(v) != 2 {
				logrus.Fatalf("incorrectly formatted control plane resource request specified: %s", rawRequest)
			}
			parsedRequestsLimits[v[0]+"-request"] = v[1]
		}
	}

	if cfg.ControlPlaneResourceLimits != "" {
		for _, rawLimit := range strings.Split(cfg.ControlPlaneResourceLimits, ",") {
			v := strings.SplitN(rawLimit, "=", 2)
			if len(v) != 2 {
				logrus.Fatalf("incorrectly formatted control plane resource request specified: %s", rawLimit)
			}
			parsedRequestsLimits[v[0]+"-limit"] = v[1]
		}
	}

	for component, request := range resources {
		for com, target := range request {
			k := component + "-" + com
			if val, ok := parsedRequestsLimits[k]; ok {
				*target = val
			}
		}
	}

	logrus.Debugf("Parsed control plane requests/limits: %+v\n", controlPlaneResources)

	extraEnv := podexecutor.ControlPlaneEnv{
		KubeAPIServer:          cfg.ExtraEnv.KubeAPIServer.Value(),
		KubeScheduler:          cfg.ExtraEnv.KubeScheduler.Value(),
		KubeControllerManager:  cfg.ExtraEnv.KubeControllerManager.Value(),
		KubeProxy:              cfg.ExtraEnv.KubeProxy.Value(),
		Etcd:                   cfg.ExtraEnv.Etcd.Value(),
		CloudControllerManager: cfg.ExtraEnv.CloudControllerManager.Value(),
	}

	extraMounts := podexecutor.ControlPlaneMounts{
		KubeAPIServer:          cfg.ExtraMounts.KubeAPIServer.Value(),
		KubeScheduler:          cfg.ExtraMounts.KubeScheduler.Value(),
		KubeControllerManager:  cfg.ExtraMounts.KubeControllerManager.Value(),
		KubeProxy:              cfg.ExtraMounts.KubeProxy.Value(),
		Etcd:                   cfg.ExtraMounts.Etcd.Value(),
		CloudControllerManager: cfg.ExtraMounts.CloudControllerManager.Value(),
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
		ControlPlaneMounts:    extraMounts,
	}, nil
}

func hostnameFQDN() (string, error) {
	cmd := exec.Command("hostname", "-f")

	var b bytes.Buffer
	cmd.Stdout = &b

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(b.String()), nil
}
