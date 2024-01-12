//go:build windows
// +build windows

package pebinaryexecutor

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Microsoft/hcsshim/hcn"
	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/rancher/rke2/pkg/bootstrap"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/logging"
	win "github.com/rancher/rke2/pkg/windows"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	ssldirs = []string{
		"/etc/ssl/certs",
		"/etc/pki/tls/certs",
		"/etc/ca-certificates",
		"/usr/local/share/ca-certificates",
		"/usr/share/ca-certificates",
	}
)

type PEBinaryConfig struct {
	ManifestsDir        string
	ImagesDir           string
	Resolver            *images.Resolver
	CloudProvider       *CloudProviderConfig
	CISMode             bool
	DataDir             string
	AuditPolicyFile     string
	KubeletPath         string
	KubeConfigKubeProxy string
	DisableETCD         bool
	IsServer            bool
	CNIName             string
	CNIPlugin           win.CNIPlugin
}

type CloudProviderConfig struct {
	Name string
	Path string
}

const (
	CNINone   = "none"
	CNICalico = "calico"
	CNICilium = "cilium"
	CNICanal  = "canal"
)

// Bootstrap prepares the binary executor to run components by setting the system default registry
// and staging the kubelet and containerd binaries.  On servers, it also ensures that manifests are
// copied in to place and in sync with the system configuration.
func (p *PEBinaryConfig) Bootstrap(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	// On servers this is set to an initial value from the CLI when the resolver is created, so that
	// static pod manifests can be created before the agent bootstrap is complete. The agent itself
	// really only needs to know about the runtime and pause images, all of which are configured after the
	// default registry has been set by the server.
	if nodeConfig.AgentConfig.SystemDefaultRegistry != "" {
		if err := p.Resolver.ParseAndSetDefaultRegistry(nodeConfig.AgentConfig.SystemDefaultRegistry); err != nil {
			return err
		}
	}

	pauseImage, err := p.Resolver.GetReference(images.Pause)
	if err != nil {
		return err
	}
	nodeConfig.AgentConfig.PauseImage = pauseImage.Name()

	setWindowsAgentSpecificSettings(p.DataDir, nodeConfig)
	// stage bootstrap content from runtime image
	execPath, err := bootstrap.Stage(p.Resolver, nodeConfig, cfg)
	if err != nil {
		return err
	}
	if err := os.Setenv("PATH", execPath+":"+os.Getenv("PATH")); err != nil {
		return err
	}

	if p.IsServer {
		return bootstrap.UpdateManifests(p.Resolver, nodeConfig, cfg)
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", nodeConfig.AgentConfig.KubeConfigK3sController)

	p.CNIName, err = getCNIPluginName(restConfig)
	if err != nil {
		return err
	}

	switch p.CNIName {
	case "", CNICalico:
		p.CNIPlugin = &win.Calico{}
		if err := p.CNIPlugin.Setup(ctx, nodeConfig, restConfig, p.DataDir); err != nil {
			return err
		}
	case CNINone:
		logrus.Info("Skipping CNI setup")
	default:
		logrus.Fatal("Unsupported CNI: ", p.CNIName)
	}

	// required to initialize KubeProxy
	p.KubeConfigKubeProxy = nodeConfig.AgentConfig.KubeConfigKubeProxy

	logrus.Infof("Windows bootstrap okay. Exiting setup.")
	return nil
}

// Kubelet starts the kubelet in a subprocess with watching goroutine.
func (p *PEBinaryConfig) Kubelet(ctx context.Context, args []string) error {
	extraArgs := map[string]string{
		"file-check-frequency":     "5s",
		"sync-frequency":           "30s",
		"cgroups-per-qos":          "false",
		"enforce-node-allocatable": "",
		"resolv-conf":              "",
		"hairpin-mode":             "promiscuous-bridge",
	}
	if p.CloudProvider != nil {
		extraArgs["cloud-provider"] = p.CloudProvider.Name
		extraArgs["cloud-config"] = p.CloudProvider.Path
	}

	args = append(getArgs(extraArgs), args...)
	args, logOut := logging.ExtractFromArgs(args)

	var cleanArgs []string
	for _, arg := range args {
		if strings.Contains(arg, "eviction-hard") || strings.Contains(arg, "eviction-minimum-reclaim") {
			continue
		}
		cleanArgs = append(cleanArgs, arg)
	}

	logrus.Infof("Running RKE2 kubelet %v", cleanArgs)
	go func() {
		for {
			cniCtx, cancel := context.WithCancel(ctx)
			if p.CNIName != CNINone {
				go func() {
					if err := p.CNIPlugin.Start(cniCtx); err != nil {
						logrus.Errorf("error in cni start: %s", err)
					}
				}()
			}

			cmd := exec.CommandContext(ctx, p.KubeletPath, cleanArgs...)
			cmd.Stdout = logOut
			cmd.Stderr = logOut
			if err := cmd.Run(); err != nil {
				logrus.Errorf("Kubelet exited: %v", err)
			}
			cancel()
			time.Sleep(5 * time.Second)
		}
	}()
	return nil
}

// KubeProxy starts the kubeproxy in a subprocess with watching goroutine.
func (p *PEBinaryConfig) KubeProxy(ctx context.Context, args []string) error {
	if p.CNIName == CNINone {
		return nil
	}

	CNIConfig := p.CNIPlugin.GetConfig()
	vip, err := p.CNIPlugin.ReserveSourceVip(ctx)
	if err != nil || vip == "" {
		logrus.Errorf("Failed to reserve VIP for kube-proxy: %s", err)
	}
	logrus.Infof("Reserved VIP for kube-proxy: %s", vip)


	extraArgs := map[string]string{
		"network-name": CNIConfig.OverlayNetName,
		"bind-address": CNIConfig.NodeIP,
		"source-vip": vip,
	}

	if err := hcn.DSRSupported(); err == nil {
		logrus.Infof("WinDSR support is enabled")
		extraArgs["feature-gates"] = addFeatureGate(extraArgs["feature-gates"], "WinDSR=true")
		extraArgs["enable-dsr"] = "true"
	}

	args = append(getArgs(extraArgs), args...)

	logrus.Infof("Running RKE2 kube-proxy %s", args)
	go func() {
		outputFile := logging.GetLogger(filepath.Join(p.DataDir, "agent", "logs", "kube-proxy.log"), 50)
		for {
			cmd := exec.CommandContext(ctx, filepath.Join("c:\\", p.DataDir, "bin", "kube-proxy.exe"), args...)
			cmd.Stdout = outputFile
			cmd.Stderr = outputFile
			err := cmd.Run()
			logrus.Errorf("kube-proxy exited: %v", err)
			time.Sleep(5 * time.Second)
		}
	}()

	return nil
}

// APIServerHandlers isn't supported in the binary executor.
func (p *PEBinaryConfig) APIServerHandlers(ctx context.Context) (authenticator.Request, http.Handler, error) {
	panic("kube-api-server is unsupported on windows")
}

// APIServer isn't supported in the binary executor.
func (p *PEBinaryConfig) APIServer(ctx context.Context, etcdReady <-chan struct{}, args []string) error {
	panic("kube-api-server is unsupported on windows")
}

// Scheduler isn't supported in the binary executor.
func (p *PEBinaryConfig) Scheduler(ctx context.Context, apiReady <-chan struct{}, args []string) error {
	panic("kube-scheduler is unsupported on windows")
}

// ControllerManager isn't supported in the binary executor.
func (p *PEBinaryConfig) ControllerManager(ctx context.Context, apiReady <-chan struct{}, args []string) error {
	panic("kube-controller-manager is unsupported on windows")
}

// CurrentETCDOptions isn't supported in the binary executor.
func (p *PEBinaryConfig) CurrentETCDOptions() (opts executor.InitialOptions, err error) {
	panic("etcd options are unsupported on windows")
}

// CloudControllerManager isn't supported in the binary executor.
func (p *PEBinaryConfig) CloudControllerManager(ctx context.Context, ccmRBACReady <-chan struct{}, args []string) error {
	panic("cloud-controller-manager is unsupported on windows.")
}

// ETCD isn't supported in the binary executor.
func (p *PEBinaryConfig) ETCD(ctx context.Context, args executor.ETCDConfig, extraArgs []string) error {
	panic("etcd is unsupported on windows")
}

// addFeatureGate adds a feature gate with the correct syntax.
func addFeatureGate(current, new string) string {
	if current == "" {
		return new
	}
	return current + "," + new
}

// getArgs concerts a map to the correct args list format.
func getArgs(argsMap map[string]string) []string {
	var args []string
	for arg, value := range argsMap {
		cmd := fmt.Sprintf("--%s=%s", arg, value)
		args = append(args, cmd)
	}
	sort.Strings(args)
	return args
}

func getCNIPluginName(restConfig *rest.Config) (string, error) {
	hc, err := helm.NewFactoryFromConfig(restConfig)
	if err != nil {
		return "", err
	}
	hl, err := hc.Helm().V1().HelmChart().List(metav1.NamespaceSystem, metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, h := range hl.Items {
		switch h.Name {
		case win.CalicoChart:
			return CNICalico, nil
		case "rke2-cilium":
			return CNICilium, nil
		case "rke2-canal":
			return CNICanal, nil
		}
	}
	return CNINone, nil
}

// setWindowsAgentSpecificSettings configures the correct paths needed for Windows
func setWindowsAgentSpecificSettings(dataDir string, nodeConfig *daemonconfig.Node) {
	nodeConfig.AgentConfig.CNIBinDir = filepath.Join("c:\\", dataDir, "bin")
	nodeConfig.AgentConfig.CNIConfDir = filepath.Join("c:\\", dataDir, "agent", "etc", "cni")
}
