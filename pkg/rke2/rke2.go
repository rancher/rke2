package rke2

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

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
	"github.com/rancher/rke2/pkg/rke2/psp"
	"github.com/rancher/spur/cli"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Repo                string
	CloudProviderName   string
	CloudProviderConfig string
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

	sCtx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1)
	}()

	go func() {
		if err := psp.SetPSPs(sCtx, ctx, nil); err != nil {
			logrus.Fatal(err)
		}
	}()

	return server.Run(ctx)
}

func Agent(ctx *cli.Context, cfg Config) error {
	if err := setup(ctx, cfg); err != nil {
		return err
	}
	return agent.Run(ctx)
}

func setup(ctx *cli.Context, cfg Config) error {
	var dataDir string
	for _, f := range ctx.Command.Flags {
		switch t := f.(type) {
		case *cli.StringFlag:
			if strings.Contains(t.Name, "data-dir") {
				dataDir = t.DefaultText
			}
		}
	}

	images := images.New(cfg.Repo)
	if err := defaults.Set(ctx, images, dataDir); err != nil {
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

	sp := podexecutor.StaticPod{
		Images:        images,
		PullImages:    pullImages,
		Manifests:     manifests,
		CISMode:       cisMode,
		CloudProvider: cpConfig,
	}
	executor.Set(&sp)

	return nil
}
