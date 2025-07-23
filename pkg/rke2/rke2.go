package rke2

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/cli/agent"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/server"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	rawServer "github.com/k3s-io/k3s/pkg/server"
	pkgerrors "github.com/pkg/errors"
	rke2cli "github.com/rancher/rke2/pkg/cli"
	"github.com/rancher/rke2/pkg/controllers"
	"github.com/rancher/rke2/pkg/controllers/cisnetworkpolicy"
	"github.com/rancher/rke2/pkg/executor/staticpod"
	"github.com/rancher/wrangler/v3/pkg/slice"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Valid CIS Profile versions
const (
	ProfileCIS  = "cis"
	ProfileETCD = "etcd"
)

func Server(clx *cli.Context, cfg rke2cli.Config) error {
	serverControllers, err := setup(clx, cfg, true)
	if err != nil {
		return err
	}

	if err := clx.Set("secrets-encryption", "true"); err != nil {
		return err
	}

	// Disable all disableable k3s packaged components. In addition to manifests,
	// this also disables several integrated controllers.
	disableItems := strings.Split(cmds.DisableItems, ",")
	for _, item := range disableItems {
		item = strings.TrimSpace(item)
		if clx.Bool("enable-" + item) {
			continue
		}
		if err := clx.Set("disable", item); err != nil {
			return err
		}
	}
	cisMode := isCISMode(clx)
	defaultNamespaces := []string{
		metav1.NamespaceSystem,
		metav1.NamespaceDefault,
		metav1.NamespacePublic,
	}
	dataDir := clx.String("data-dir")
	cmds.ServerConfig.StartupHooks = append(cmds.ServerConfig.StartupHooks,
		setNetworkPolicies(cisMode, defaultNamespaces),
		setClusterRoles(),
		restrictServiceAccounts(cisMode, defaultNamespaces),
		setKubeProxyDisabled(),
		cleanupStaticPodsOnSelfDelete(dataDir),
		setRuntimes(),
	)

	var leaderControllers rawServer.CustomControllers
	var controllers rawServer.CustomControllers

	if serverControllers != nil {
		leaderControllers = serverControllers.LeaderControllers()
		controllers = serverControllers.Controllers()
	}

	cnis := clx.StringSlice("cni")
	if cisMode && (len(cnis) == 0 || slice.ContainsString(cnis, "canal")) {
		leaderControllers = append(leaderControllers, cisnetworkpolicy.Controller)
	} else {
		leaderControllers = append(leaderControllers, cisnetworkpolicy.Cleanup)
	}

	return server.RunWithControllers(clx, leaderControllers, controllers)
}

func Agent(clx *cli.Context, cfg rke2cli.Config) error {
	if _, err := setup(clx, cfg, false); err != nil {
		return err
	}
	return agent.Run(clx)
}

func setup(clx *cli.Context, cfg rke2cli.Config, isServer bool) (controllers.Server, error) {
	// If we are pid 1, k3s is about to reexec so we shouldn't bother doing anything
	// ref: https://github.com/k3s-io/k3s/blob/v1.33.0%2Bk3s1/pkg/cli/cmds/log_linux.go#L21-L24
	if os.Getpid() == 1 && os.Getenv("_K3S_LOG_REEXEC_") != "true" {
		return nil, nil
	}

	ex, err := initExecutor(clx, cfg, isServer)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to initialize executor")
	}
	executor.Set(ex)

	// note: controllers are only run on servers, even though we
	// do the type check and return them either way.
	var serverControllers controllers.Server
	if ec, ok := ex.(controllers.Server); ok {
		serverControllers = ec
	}

	return serverControllers, nil
}

func ForceRestartFile(dataDir string) string {
	return filepath.Join(dataDir, "force-restart")
}

func etcdNameFile(dataDir string) string {
	return filepath.Join(dataDir, "server", "db", "etcd", "name")
}

func isCISMode(clx *cli.Context) bool {
	profile := clx.String("profile")
	return profile == ProfileCIS
}

func profileMode(clx *cli.Context) staticpod.ProfileMode {
	switch clx.String("profile") {
	case ProfileCIS:
		return staticpod.ProfileModeCIS
	case ProfileETCD:
		return staticpod.ProfileModeETCD
	default:
		return staticpod.ProfileModeNone
	}
}

func hostnameFromMetadataEndpoint(ctx context.Context) string {
	var token string

	// Get token, required for IMDSv2
	tokenCtx, tokenCancel := context.WithTimeout(ctx, time.Second)
	defer tokenCancel()
	if req, err := http.NewRequestWithContext(tokenCtx, http.MethodPut, "http://169.254.169.254/latest/api/token", nil); err != nil {
		logrus.Debugf("Failed to create request for token endpoint: %v", err)
	} else {
		req.Header.Add("x-aws-ec2-metadata-token-ttl-seconds", "60")
		if resp, err := http.DefaultClient.Do(req); err != nil {
			logrus.Debugf("Failed to get token from token endpoint: %v", err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Debugf("Token endpoint returned unacceptable status code %d", resp.StatusCode)
			} else {
				if b, err := ioutil.ReadAll(resp.Body); err != nil {
					logrus.Debugf("Failed to read response body from token endpoint: %v", err)
				} else {
					token = string(b)
				}
			}
		}
	}

	// Get hostname from IMDS, with token if available
	metaCtx, metaCancel := context.WithTimeout(ctx, time.Second)
	defer metaCancel()
	if req, err := http.NewRequestWithContext(metaCtx, http.MethodGet, "http://169.254.169.254/latest/meta-data/local-hostname", nil); err != nil {
		logrus.Debugf("Failed to create request for metadata endpoint: %v", err)
	} else {
		if token != "" {
			req.Header.Add("x-aws-ec2-metadata-token", token)
		}
		if resp, err := http.DefaultClient.Do(req); err != nil {
			logrus.Debugf("Failed to get hostname from metadata endpoint: %v", err)
		} else {
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Debugf("Metadata endpoint returned unacceptable status code %d", resp.StatusCode)
			} else {
				if b, err := ioutil.ReadAll(resp.Body); err != nil {
					logrus.Debugf("Failed to read response body from metadata endpoint: %v", err)
				} else {
					return strings.TrimSpace(string(b))
				}
			}
		}
	}
	return ""
}
