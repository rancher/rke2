package rke2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/k3s/pkg/agent/config"
	"github.com/rancher/k3s/pkg/cli/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cli/server"
	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/rancher/k3s/pkg/etcd"
	"github.com/rancher/rke2/pkg/bootstrap"
	"github.com/rancher/rke2/pkg/cli/defaults"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/podexecutor"
	"github.com/urfave/cli"
)

type Config struct {
	SystemDefaultRegistry string
	CloudProviderName     string
	CloudProviderConfig   string
	CNIPlugin             string
}

var cisMode bool

const CISProfile = "cis-1.5"

func Server(clx *cli.Context, cfg Config) error {
	if err := setup(clx, cfg); err != nil {
		return err
	}

	if err := clx.Set("secrets-encryption", "true"); err != nil {
		return err
	}

	// Disable all disableable k3s packaged components. In addition to manifests,
	// this also disables several integrated controllers.
	disableItems := strings.Split(cmds.DisableItems, ",")
	for _, item := range disableItems {
		if err := clx.Set("disable", strings.TrimSpace(item)); err != nil {
			return err
		}
	}

	cmds.ServerConfig.StartupHooks = append(cmds.ServerConfig.StartupHooks,
		setPSPs(),
		setNetworkPolicies(),
		setClusterRoles(),
	)

	return server.Run(clx)
}

func Agent(clx *cli.Context, cfg Config) error {
	if err := setup(clx, cfg); err != nil {
		return err
	}
	return agent.Run(clx)
}

func setup(clx *cli.Context, cfg Config) error {
	cisMode = clx.String("profile") == CISProfile
	dataDir := clx.String("data-dir")

	images := images.New(cfg.SystemDefaultRegistry)
	if err := defaults.Set(clx, images, dataDir); err != nil {
		return err
	}

	execPath, err := bootstrap.Stage(dataDir, cfg.CNIPlugin, images)
	if err != nil {
		return err
	}

	if err := os.Setenv("PATH", execPath+":"+os.Getenv("PATH")); err != nil {
		return err
	}

	agentManifestsDir := filepath.Join(dataDir, "agent", config.DefaultPodManifestPath)
	agentImagesDir := filepath.Join(dataDir, "agent", "images")

	managed.RegisterDriver(&etcd.ETCD{})

	var cpConfig *podexecutor.CloudProviderConfig
	if cfg.CloudProviderConfig != "" && cfg.CloudProviderName == "" {
		return fmt.Errorf("--cloud-provider-config requires --cloud-provider-name to be provided")
	}
	if cfg.CloudProviderName != "" {
		cpConfig = &podexecutor.CloudProviderConfig{
			Name: cfg.CloudProviderName,
			Path: cfg.CloudProviderConfig,
		}
	}

	sp := podexecutor.StaticPodConfig{
		Images:        images,
		ImagesDir:     agentImagesDir,
		ManifestsDir:  agentManifestsDir,
		CISMode:       cisMode,
		CloudProvider: cpConfig,
		DataDir:       dataDir,
	}
	executor.Set(&sp)

	return nil
}
