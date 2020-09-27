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
}

var cisMode bool

func Server(clx *cli.Context, cfg Config) error {
	if err := setup(clx, cfg); err != nil {
		return err
	}
	if err := clx.Set("secrets-encryption", "true"); err != nil {
		return err
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
	var dataDir string

	for _, f := range clx.Command.Flags {
		switch t := f.(type) {
		case cli.StringFlag:
			if strings.Contains(t.Name, "data-dir") {
				dataDir = *t.Destination
			}
		}
	}

	for _, f := range clx.App.Flags {
		switch t := f.(type) {
		case cli.StringFlag:
			if t.Name == "profile" && t.Destination != nil && *t.Destination != "" {
				cisMode = true
			}
		default:
			// nothing to do. Keep moving.
		}
	}

	images := images.New(cfg.SystemDefaultRegistry)
	if err := defaults.Set(clx, images, dataDir, cisMode); err != nil {
		return err
	}

	execPath, err := bootstrap.Stage(dataDir, images)
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
