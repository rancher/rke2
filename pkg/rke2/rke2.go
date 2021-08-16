package rke2

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent/config"
	"github.com/rancher/k3s/pkg/cli/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cli/etcdsnapshot"
	"github.com/rancher/k3s/pkg/cli/server"
	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/rancher/k3s/pkg/etcd"
	rawServer "github.com/rancher/k3s/pkg/server"
	"github.com/rancher/rke2/pkg/bootstrap"
	"github.com/rancher/rke2/pkg/cli/defaults"
	"github.com/rancher/rke2/pkg/controllers/cisnetworkpolicy"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/podexecutor"
	"github.com/urfave/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Config struct {
	CloudProviderName   string
	CloudProviderConfig string
	AuditPolicyFile     string
	KubeletPath         string
	Images              images.Images
}

var cisMode bool

// Valid CIS Profile versions
const (
	CISProfile             = "cis-1.5"
	defaultAuditPolicyFile = "/etc/rancher/rke2/audit-policy.yaml"
)

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
	var defaultNamespaces = []string{
		metav1.NamespaceSystem,
		metav1.NamespaceDefault,
		metav1.NamespacePublic,
	}
	cmds.ServerConfig.StartupHooks = append(cmds.ServerConfig.StartupHooks,
		setPSPs(),
		setNetworkPolicies(defaultNamespaces),
		setClusterRoles(),
		restrictServiceAccounts(defaultNamespaces),
	)

	var leaderControllers rawServer.CustomControllers

	if cisMode {
		leaderControllers = append(leaderControllers, cisnetworkpolicy.Controller)
	}

	return server.RunWithControllers(clx, leaderControllers, rawServer.CustomControllers{})
}

func Agent(clx *cli.Context, cfg Config) error {
	if err := setup(clx, cfg); err != nil {
		return err
	}
	return agent.Run(clx)
}

func EtcdSnapshot(clx *cli.Context, cfg Config) error {
	cmds.ServerConfig.DatastoreEndpoint = "etcd"
	return etcdsnapshot.Run(clx)
}

func setup(clx *cli.Context, cfg Config) error {
	cisMode = clx.String("profile") == CISProfile
	dataDir := clx.String("data-dir")

	auditPolicyFile := clx.String("audit-policy-file")
	if auditPolicyFile == "" {
		auditPolicyFile = defaultAuditPolicyFile
	}

	cfg.Images.SetDefaults()
	if err := defaults.Set(clx, cfg.Images, dataDir); err != nil {
		return err
	}

	execPath, err := bootstrap.Stage(dataDir, cfg.Images)
	if err != nil {
		return err
	}

	if err := os.Setenv("PATH", execPath+":"+os.Getenv("PATH")); err != nil {
		return err
	}

	agentManifestsDir := filepath.Join(dataDir, "agent", config.DefaultPodManifestPath)
	agentImagesDir := filepath.Join(dataDir, "agent", "images")

	managed.RegisterDriver(&etcd.ETCD{})

	if clx.IsSet("node-external-ip") {
		if clx.IsSet("cloud-provider-config") || clx.IsSet("cloud-provider-name") {
			return errors.New("can't set node-external-ip while using cloud provider")
		}
		cmds.ServerConfig.DisableCCM = false
	}
	var cpConfig *podexecutor.CloudProviderConfig
	if cfg.CloudProviderConfig != "" && cfg.CloudProviderName == "" {
		return fmt.Errorf("--cloud-provider-config requires --cloud-provider-name to be provided")
	}
	if cfg.CloudProviderName != "" {
		cpConfig = &podexecutor.CloudProviderConfig{
			Name: cfg.CloudProviderName,
			Path: cfg.CloudProviderConfig,
		}
		if clx.String("node-name") == "" && cfg.CloudProviderName == "aws" {
			fqdn, err := hostnameFQDN()
			if err != nil {
				return err
			}
			if err := clx.Set("node-name", fqdn); err != nil {
				return err
			}
		}
	}

	if cfg.KubeletPath == "" {
		cfg.KubeletPath = "kubelet"
	}

	sp := podexecutor.StaticPodConfig{
		Images:          cfg.Images,
		ImagesDir:       agentImagesDir,
		ManifestsDir:    agentManifestsDir,
		CISMode:         cisMode,
		CloudProvider:   cpConfig,
		DataDir:         dataDir,
		AuditPolicyFile: auditPolicyFile,
		KubeletPath:     cfg.KubeletPath,
	}
	executor.Set(&sp)

	return nil
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
