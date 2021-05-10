package rke2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	containerdk3s "github.com/rancher/k3s/pkg/agent/containerd"
	"github.com/rancher/rke2/pkg/controllers/cisnetworkpolicy"
	"github.com/sirupsen/logrus"

	"github.com/rancher/k3s/pkg/agent/config"
	"github.com/rancher/k3s/pkg/cli/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cli/etcdsnapshot"
	"github.com/rancher/k3s/pkg/cli/server"
	"github.com/rancher/k3s/pkg/cluster/managed"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/rancher/k3s/pkg/etcd"
	rawServer "github.com/rancher/k3s/pkg/server"
	"github.com/rancher/rke2/pkg/cli/defaults"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/podexecutor"
	"github.com/urfave/cli"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

type Config struct {
	AuditPolicyFile     string
	CloudProviderConfig string
	CloudProviderName   string
	Images              images.ImageOverrideConfig
	KubeletPath         string
}

const (
	CISProfile15           = "cis-1.5"
	CISProfile16           = "cis-1.6"
	defaultAuditPolicyFile = "/etc/rancher/rke2/audit-policy.yaml"
)

func Server(clx *cli.Context, cfg Config) error {
	if err := setup(clx, cfg, true); err != nil {
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
	cisMode := isCISMode(clx)

	cmds.ServerConfig.StartupHooks = append(cmds.ServerConfig.StartupHooks,
		setPSPs(cisMode),
		setNetworkPolicies(cisMode),
		setClusterRoles(),
	)

	var leaderControllers rawServer.CustomControllers

	if cisMode {
		leaderControllers = append(leaderControllers, cisnetworkpolicy.CISNetworkPolicyController)
	}

	return server.RunWithControllers(clx, leaderControllers, rawServer.CustomControllers{})
}

func Agent(clx *cli.Context, cfg Config) error {
	if err := setup(clx, cfg, false); err != nil {
		return err
	}
	return agent.Run(clx)
}

func EtcdSnapshot(clx *cli.Context, cfg Config) error {
	cmds.ServerConfig.DatastoreEndpoint = "etcd"
	return etcdsnapshot.Run(clx)
}

func setup(clx *cli.Context, cfg Config, isServer bool) error {
	dataDir := clx.String("data-dir")
	disableETCD := clx.Bool("disable-etcd")
	disableScheduler := clx.Bool("disable-scheduler")
	disableAPIServer := clx.Bool("disable-apiserver")
	disableControllerManager := clx.Bool("disable-controller-manager")
	clusterReset := clx.Bool("cluster-reset")

	auditPolicyFile := clx.String("audit-policy-file")
	if auditPolicyFile == "" {
		auditPolicyFile = defaultAuditPolicyFile
	}

	// This flag will only be set on servers, on agents this is a no-op and the
	// resolver's default registry will get updated later when bootstrapping
	cfg.Images.SystemDefaultRegistry = clx.String("system-default-registry")
	resolver, err := images.NewResolver(cfg.Images)
	if err != nil {
		return err
	}

	if err := defaults.Set(clx, dataDir); err != nil {
		return err
	}

	agentManifestsDir := filepath.Join(dataDir, "agent", config.DefaultPodManifestPath)
	agentImagesDir := filepath.Join(dataDir, "agent", "images")

	managed.RegisterDriver(&etcd.ETCD{})

	if clx.IsSet("cloud-provider-config") || clx.IsSet("cloud-provider-name") {
		if clx.IsSet("node-external-ip") {
			return errors.New("can't set node-external-ip while using cloud provider")
		}
		cmds.ServerConfig.DisableCCM = true
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
	}

	if cfg.KubeletPath == "" {
		cfg.KubeletPath = "kubelet"
	}

	sp := podexecutor.StaticPodConfig{
		Resolver:        resolver,
		ImagesDir:       agentImagesDir,
		ManifestsDir:    agentManifestsDir,
		CISMode:         isCISMode(clx),
		CloudProvider:   cpConfig,
		DataDir:         dataDir,
		AuditPolicyFile: auditPolicyFile,
		KubeletPath:     cfg.KubeletPath,
		DisableETCD:     disableETCD,
		IsServer:        isServer,
	}
	executor.Set(&sp)

	disabledItems := map[string]bool{
		"kube-apiserver":          disableAPIServer,
		"kube-scheduler":          disableScheduler,
		"kube-controller-manager": disableControllerManager,
		"etcd":                    disableETCD,
	}
	return removeOldPodManifests(dataDir, disabledItems, clusterReset)
}

func podManifestsDir(dataDir string) string {
	return filepath.Join(dataDir, "agent", config.DefaultPodManifestPath)
}

func removeOldPodManifests(dataDir string, disabledItems map[string]bool, clusterReset bool) error {
	var kubeletStandAlone bool

	kubeletErr := make(chan error)
	containerdErr := make(chan error)

	ctx, cancel := context.WithTimeout(context.Background(), (5 * time.Minute))
	defer cancel()

	manifestDir := podManifestsDir(dataDir)
	for component, disabled := range disabledItems {
		if disabled {
			manifestName := filepath.Join(manifestDir, component+".yaml")
			if _, err := os.Stat(manifestName); err == nil {
				kubeletStandAlone = true
			}
		}
	}

	if clusterReset {
		// deleting old etcd if cluster reset is passed
		manifestName := filepath.Join(manifestDir, "etcd.yaml")
		if _, err := os.Stat(manifestName); err == nil {
			kubeletStandAlone = true
		}
	}
	if kubeletStandAlone {
		// delete all manifests
		for component := range disabledItems {
			manifestName := filepath.Join(manifestDir, component+".yaml")
			if err := os.RemoveAll(manifestName); err != nil {
				return err
			}
		}
		kubeletCmd := exec.CommandContext(ctx, "kubelet")
		containerdCmd := exec.CommandContext(ctx, "containerd")

		tmpAddress := filepath.Join(os.TempDir(), "containerd.sock")

		// start containerd
		go startContainerd(dataDir, containerdErr, tmpAddress, containerdCmd)
		// start kubelet
		go startKubelet(dataDir, kubeletErr, tmpAddress, kubeletCmd)

		// check for any running containers from the disabled items list
		go checkForRunningContainers(ctx, tmpAddress, disabledItems, kubeletErr, containerdErr)

		for {
			time.Sleep(5 * time.Second)
			select {
			case err := <-kubeletErr:
				logrus.Infof("kubelet Exited: %v, exiting Containerd", err)
				// exits containerd if kubelet exits
				containerdCmd.Process.Kill()
				kubeletCmd.Process.Kill()

			case err := <-containerdErr:
				logrus.Infof("Containerd Exited: %v, exiting kubelet", err)
				kubeletCmd.Process.Kill()
				containerdCmd.Process.Kill()

			case <-ctx.Done():
				logrus.Info("Timeout reached, exiting kubelet and containerd")
				kubeletCmd.Process.Kill()
				containerdCmd.Process.Kill()
			}
			break
		}
	}

	return nil
}

func isCISMode(clx *cli.Context) bool {
	profile := clx.String("profile")
	return profile == CISProfile15 || profile == CISProfile16
}

func startKubelet(dataDir string, errChan chan error, tmpAddress string, cmd *exec.Cmd) {
	logrus.Infof("starting kubelet to clean up old static pods")
	args := []string{
		"--fail-swap-on=false",
		"--container-runtime=remote",
		"--containerd=" + tmpAddress,
		"--container-runtime-endpoint=unix://" + tmpAddress,
		"--pod-manifest-path=" + podManifestsDir(dataDir),
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	errChan <- cmd.Run()
}

func startContainerd(dataDir string, errChan chan error, tmpAddress string, cmd *exec.Cmd) {
	args := []string{
		"-a", tmpAddress,
		"--root", filepath.Join(dataDir, "agent", "containerd"),
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	errChan <- cmd.Run()
}

func isContainerRunning(ctx context.Context, name string, resp *runtimeapi.ListContainersResponse) bool {
	for _, c := range resp.Containers {
		if c.Metadata.Name == name {
			return true
		}
	}
	return false
}

func checkForRunningContainers(ctx context.Context, tmpAddress string, disabledItems map[string]bool, kubeletErr, containerdErr chan error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		conn, err := containerdk3s.CriConnection(ctx, tmpAddress)
		if err != nil {
			logrus.Warnf("Failed to setup cri connection: %v", err)
			continue
		}
		c := runtimeapi.NewRuntimeServiceClient(conn)
		defer conn.Close()
		resp, err := c.ListContainers(ctx, &runtimeapi.ListContainersRequest{})
		if err != nil {
			logrus.Warnf("Failed to list containers: %v", err)
			continue
		}
		var gc bool
		for item, _ := range disabledItems {
			if isContainerRunning(ctx, item, resp) {
				logrus.Infof("Waiting for deletion of %s static pod", item)
				gc = true
				break
			}
		}
		if gc {
			continue
		}
		// if all disabled items containers has been removed
		// then terminate kubelet and containerd
		containerdErr <- fmt.Errorf("static pods deleted")
		kubeletErr <- fmt.Errorf("static pods deleted")
		break
	}
}
