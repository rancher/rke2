//go:build windows
// +build windows

package rke2

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"unsafe"

	"github.com/k3s-io/k3s/pkg/agent/config"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/etcd"
	rke2cli "github.com/rancher/rke2/pkg/cli"
	"github.com/rancher/rke2/pkg/cli/defaults"
	"github.com/rancher/rke2/pkg/executor/pebinary"
	"github.com/rancher/rke2/pkg/images"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/sys/windows"
)

func initExecutor(clx *cli.Context, cfg rke2cli.Config, isServer bool) (executor.Executor, error) {
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
	var cpConfig *pebinary.CloudProviderConfig
	if cfg.CloudProviderConfig != "" && cfg.CloudProviderName == "" {
		return nil, fmt.Errorf("--cloud-provider-config requires --cloud-provider-name to be provided")
	}
	if cfg.CloudProviderName != "" {
		if cfg.CloudProviderName == "aws" {
			logrus.Warnf("--cloud-provider-name=aws is deprecated due to removal of the in-tree aws cloud provider; if you want the legacy node-name behavior associated with this flag please use --node-name-from-cloud-provider-metadata")
			cfg.CloudProviderMetadataHostname = true
			cfg.CloudProviderName = ""
		} else {
			cpConfig = &pebinary.CloudProviderConfig{
				Name: cfg.CloudProviderName,
				Path: cfg.CloudProviderConfig,
			}
		}
	}

	if cfg.CloudProviderMetadataHostname {
		fqdn := hostnameFromMetadataEndpoint(context.Background())
		if fqdn == "" {
			hostFQDN, err := hostnameFQDN()
			if err != nil {
				return nil, err
			}
			fqdn = hostFQDN
		}
		if err := clx.Set("node-name", fqdn); err != nil {
			return nil, err
		}
	}

	if cfg.KubeletPath == "" {
		cfg.KubeletPath = "kubelet"
	}

	var ingressControllerName string
	if len(cfg.IngressController.Value()) > 0 {
		ingressControllerName = cfg.IngressController.Value()[0]
	}

	return &pebinary.PEBinaryConfig{
		Resolver:          resolver,
		ImagesDir:         agentImagesDir,
		ManifestsDir:      agentManifestsDir,
		CISMode:           isCISMode(clx),
		CloudProvider:     cpConfig,
		DataDir:           dataDir,
		AuditPolicyFile:   clx.String("audit-policy-file"),
		KubeletPath:       cfg.KubeletPath,
		DisableETCD:       clx.Bool("disable-etcd"),
		IsServer:          isServer,
		IngressController: ingressControllerName,
		CNIName:           "",
	}, nil
}

func hostnameFQDN() (string, error) {
	var domainName *uint16
	var domainNameLen uint32 = 256
	err := windows.GetComputerNameEx(windows.ComputerNameDnsFullyQualified, domainName, &domainNameLen)
	if err != nil {
		return "", err
	}
	return windows.UTF16ToString((*[1 << 16]uint16)(unsafe.Pointer(domainName))[:domainNameLen-1]), nil
}
