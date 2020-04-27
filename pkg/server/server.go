package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher/rke2/pkg/bootstrap"

	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cli/server"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/rancher/rke2/pkg/cli/defaults"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/podexecutor"
	"github.com/urfave/cli"
)

type Config struct {
	Version string
	Repo    string
}

func Run(ctx *cli.Context, cfg Config) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}

	if err := ctx.Set("disable", cmds.DisableItems); err != nil {
		return err
	}

	images := images.New(cfg.Repo, cfg.Version)
	defaults.Set(images)

	execPath, err := bootstrap.Stage(cmds.ServerConfig.DataDir, images)
	if err != nil {
		return err
	}

	if err := os.Setenv("PATH", fmt.Sprintf("%s:%s", execPath, os.Getenv("PATH"))); err != nil {
		return err
	}

	executor.Set(&podexecutor.StaticPod{
		Images:    images,
		Manifests: filepath.Join(cmds.ServerConfig.DataDir, "agent", "manifests"),
	})

	return server.Run(ctx)
}
