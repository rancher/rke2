//go:build windows
// +build windows

package rke2

import (
	"fmt"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/agent/config"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/pkg/errors"
	"github.com/rancher/rke2/pkg/cli/defaults"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/pebinaryexecutor"
	"github.com/urfave/cli"
)

func initExecutor(clx *cli.Context, cfg Config, isServer bool) (*pebinaryexecutor.PEBinaryConfig, error) {
	// This flag will only be set on servers, on agents this is a no-op and the
	// resolver's default registry will get updated later when bootstrapping
	cfg.Images.SystemDefaultRegistry = clx.String("system-default-registry")
	resolver, err := images.NewResolver(cfg.Images)
	if err != nil {
		return nil, err
	}

	dataDir := clx.String("data-dir")
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
	var cpConfig *pebinaryexecutor.CloudProviderConfig
	if cfg.CloudProviderConfig != "" && cfg.CloudProviderName == "" {
		return nil, fmt.Errorf("--cloud-provider-config requires --cloud-provider-name to be provided")
	}
	if cfg.CloudProviderName != "" {
		cpConfig = &pebinaryexecutor.CloudProviderConfig{
			Name: cfg.CloudProviderName,
			Path: cfg.CloudProviderConfig,
		}
	}

	if cfg.KubeletPath == "" {
		cfg.KubeletPath = "kubelet"
	}

	return &pebinaryexecutor.PEBinaryConfig{
		Resolver:        resolver,
		ImagesDir:       agentImagesDir,
		ManifestsDir:    agentManifestsDir,
		CISMode:         isCISMode(clx),
		CloudProvider:   cpConfig,
		DataDir:         dataDir,
		AuditPolicyFile: clx.String("audit-policy-file"),
		KubeletPath:     cfg.KubeletPath,
		DisableETCD:     clx.Bool("disable-etcd"),
		IsServer:        isServer,
		CNIName:         "",
	}, nil
}
