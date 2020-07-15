package rke2

import (
	"os"
	"path/filepath"

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
	"github.com/rancher/spur/cli"
)

type Config struct {
	Repo string
}

func Server(ctx *cli.Context, cfg Config) error {
	if err := setup(ctx, cfg); err != nil {
		return err
	}
	if err := ctx.Set("disable", cmds.DisableItems); err != nil {
		return err
	}
	if err := ctx.Set("secrets-encryption", "true"); err != nil {
		return err
	}
	return server.Run(ctx)
}

func Agent(ctx *cli.Context, cfg Config) error {
	if err := setup(ctx, cfg); err != nil {
		return err
	}
	return agent.Run(ctx)
}

func setup(ctx *cli.Context, cfg Config) error {
	dataDir := cmds.ServerConfig.DataDir
	if dataDir == "" {
		dataDir = cmds.AgentConfig.DataDir
	}

	images := images.New(cfg.Repo)
	if err := defaults.Set(images, dataDir); err != nil {
		return err
	}

	execPath, err := bootstrap.Stage(dataDir, images)
	if err != nil {
		return err
	}

	if err := os.Setenv("PATH", execPath+":"+os.Getenv("PATH")); err != nil {
		return err
	}

	manifests := filepath.Join(dataDir, "agent", config.DefaultPodManifestPath)
	pullImages := filepath.Join(dataDir, "agent", "images")
	cisMode := ctx.String("profile") != ""

	managed.RegisterDriver(&etcd.ETCD{})

	sp := podexecutor.StaticPod{
		Images:     images,
		PullImages: pullImages,
		Manifests:  manifests,
		CISMode:    cisMode,
	}
	executor.Set(&sp)

	return nil
}
