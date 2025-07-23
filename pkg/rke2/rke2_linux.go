//go:build linux
// +build linux

package rke2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/k3s-io/k3s/pkg/agent/config"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/kine/pkg/util"
	pkgerrors "github.com/pkg/errors"
	rke2cli "github.com/rancher/rke2/pkg/cli"
	"github.com/rancher/rke2/pkg/cli/defaults"
	"github.com/rancher/rke2/pkg/executor/staticpod"
	"github.com/rancher/rke2/pkg/podtemplate"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func initExecutor(clx *cli.Context, cfg rke2cli.Config, isServer bool) (executor.Executor, error) {
	// This flag will only be set on servers, on agents this is a no-op and the
	// resolver's default registry will get updated later when bootstrapping
	cfg.Images.SystemDefaultRegistry = clx.String("system-default-registry")

	dataDir := clx.String("data-dir")
	if err := defaults.Set(clx, dataDir); err != nil {
		return nil, err
	}

	if clx.IsSet("cloud-provider-config") || clx.IsSet("cloud-provider-name") {
		if clx.IsSet("node-external-ip") {
			return nil, errors.New("can't set node-external-ip while using cloud provider")
		}
		cmds.ServerConfig.DisableCCM = true
	}

	return initStaticPodExecutor(clx, cfg, isServer)
}

func initStaticPodExecutor(clx *cli.Context, cfg rke2cli.Config, isServer bool) (executor.Executor, error) {
	// Verify if the user want to use kine as the datastore
	// and then remove the etcd from the static pod
	externalDatabase := false
	if cmds.ServerConfig.DatastoreEndpoint != "" || (clx.Bool("disable-etcd") && !clx.IsSet("server")) {
		cmds.ServerConfig.DisableETCD = false
		cmds.ServerConfig.ClusterInit = false

		// When the datastore sets a etcd endpoint, rke2 does not need kine with tls and changes
		// in the --etcd-servers inside staticpod using externalDatabase
		scheme, _ := util.SchemeAndAddress(cmds.ServerConfig.DatastoreEndpoint)
		switch scheme {
		case "http", "https":
		default:
			cmds.ServerConfig.KineTLS = true
			externalDatabase = true
		}
	} else {
		managed.RegisterDriver(&etcd.ETCD{})
	}

	dataDir := clx.String("data-dir")
	clusterReset := clx.Bool("cluster-reset")
	clusterResetRestorePath := clx.String("cluster-reset-restore-path")
	agentManifestsDir := filepath.Join(dataDir, "agent", config.DefaultPodManifestPath)

	// check for force restart file
	var forceRestart bool
	if _, err := os.Stat(ForceRestartFile(dataDir)); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		forceRestart = true
		os.Remove(ForceRestartFile(dataDir))
	}

	// check for missing db name file on a server running etcd, indicating we're rejoining after cluster reset on a different node
	if _, err := os.Stat(etcdNameFile(dataDir)); err != nil && os.IsNotExist(err) && isServer && !clx.Bool("disable-etcd") && !clx.IsSet("datastore-endpoint") {
		clusterReset = true
	}

	// adding force restart file when cluster reset restore path is passed
	if clusterResetRestorePath != "" {
		forceRestartFile := ForceRestartFile(dataDir)
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(forceRestartFile, []byte(""), 0600); err != nil {
			return nil, err
		}
	}

	var cpConfig *staticpod.CloudProviderConfig
	if cfg.CloudProviderConfig != "" && cfg.CloudProviderName == "" {
		return nil, fmt.Errorf("--cloud-provider-config requires --cloud-provider-name to be provided")
	}
	if cfg.CloudProviderName != "" {
		if cfg.CloudProviderName == "aws" {
			logrus.Warnf("--cloud-provider-name=aws is deprecated due to removal of the in-tree aws cloud provider; if you want the legacy hostname behavior associated with this flag please use --node-name-from-cloud-provider-metadata")
			cfg.CloudProviderMetadataHostname = true
			cfg.CloudProviderName = ""
		} else {
			cpConfig = &staticpod.CloudProviderConfig{
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

	templateConfig, err := podtemplate.NewConfigFromCLI(dataDir, cfg)
	if err != nil {
		return nil, pkgerrors.WithMessage(err, "failed to parse pod template config")
	}

	// Adding PSAs
	podSecurityConfigFile := clx.String("pod-security-admission-config-file")
	if podSecurityConfigFile == "" {
		if err := setPSAs(isCISMode(clx)); err != nil {
			return nil, err
		}
		podSecurityConfigFile = defaultPSAConfigFile
	}

	containerRuntimeEndpoint := cmds.AgentConfig.ContainerRuntimeEndpoint
	if containerRuntimeEndpoint == "" {
		containerRuntimeEndpoint = staticpod.ContainerdSock
	}

	var ingressControllerName string
	if len(cfg.IngressController.Value()) > 0 {
		ingressControllerName = cfg.IngressController.Value()[0]
	}

	disabledItems := map[string]bool{
		"cloud-controller-manager": !isServer || forceRestart || clx.Bool("disable-cloud-controller"),
		"etcd":                     !isServer || forceRestart || clx.Bool("disable-etcd") || clx.IsSet("datastore-endpoint"),
		"kube-apiserver":           !isServer || forceRestart || clx.Bool("disable-apiserver"),
		"kube-controller-manager":  !isServer || forceRestart || clx.Bool("disable-controller-manager"),
		"kube-scheduler":           !isServer || forceRestart || clx.Bool("disable-scheduler"),
	}

	if err := staticpod.RemoveDisabledPods(dataDir, containerRuntimeEndpoint, disabledItems, clusterReset); err != nil {
		return nil, err
	}

	return &staticpod.StaticPodConfig{
		Config:            *templateConfig,
		ManifestsDir:      agentManifestsDir,
		ProfileMode:       profileMode(clx),
		CloudProvider:     cpConfig,
		AuditPolicyFile:   clx.String("audit-policy-file"),
		PSAConfigFile:     podSecurityConfigFile,
		KubeletPath:       cfg.KubeletPath,
		RuntimeEndpoint:   containerRuntimeEndpoint,
		DisableETCD:       clx.Bool("disable-etcd"),
		ExternalDatabase:  externalDatabase,
		IsServer:          isServer,
		IngressController: ingressControllerName,
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
