package cmds

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	debug      bool
	appName    = filepath.Base(os.Args[0])
	commonFlag = []cli.Flag{
		cli.StringFlag{
			Name:        "repo",
			Usage:       "(image) Image repository override for for RKE2 images",
			EnvVar:      "RKE2_REPO",
			Destination: &config.Repo,
		},
	}
)

const (
	rke2Path       = "/var/lib/rancher/rke2"
	rke2ServerPath = rke2Path + "/server"
)

func init() {
	// hack - force "file,dns" lookup order if go dns is used
	if os.Getenv("RES_OPTIONS") == "" {
		os.Setenv("RES_OPTIONS", " ")
	}
}

func NewApp() *cli.App {
	app := cli.NewApp()
	app.Name = appName
	app.Usage = "Rancher Kubernetes Engine 2"
	app.Version = fmt.Sprintf("%s (%s)", version.Version, version.GitCommit)
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s version %s\n", app.Name, app.Version)
	}
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "Turn on debug logs",
			Destination: &debug,
			EnvVar:      "K3S_DEBUG",
		},
	}

	app.Before = func(ctx *cli.Context) error {
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}

	return app
}
