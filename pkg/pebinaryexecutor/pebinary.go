//go:build windows
// +build windows

package pebinaryexecutor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Microsoft/hcsshim/hcn"
	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	"github.com/k3s-io/k3s/pkg/agent/containerd"
	"github.com/k3s-io/k3s/pkg/agent/cri"
	"github.com/k3s-io/k3s/pkg/agent/cridockerd"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/rancher/rke2/pkg/bootstrap"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/logging"
	win "github.com/rancher/rke2/pkg/windows"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
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
	CNIPlugin           win.CNIPlugin
	CloudProvider       *CloudProviderConfig
	Resolver            *images.Resolver
	ManifestsDir        string
	DataDir             string
	AuditPolicyFile     string
	KubeletPath         string
	CNIName             string
	ImagesDir           string
	KubeConfigKubeProxy string
	IngressController   string
	CISMode             bool
	DisableETCD         bool
	IsServer            bool
}

type CloudProviderConfig struct {
	Name string
	Path string
}

const (
	CNINone    = "none"
	CNICalico  = "calico"
	CNICilium  = "cilium"
	CNICanal   = "canal"
	CNIFlannel = "flannel"
)

// Bootstrap prepares the binary executor to run components by setting the system default registry
// and staging the kubelet and containerd binaries.  On servers, it also ensures that manifests are
// copied in to place and in sync with the system configuration.
func (p *PEBinaryConfig) Bootstrap(ctx context.Context, nodeConfig *config.Node, cfg cmds.Agent) error {
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
	if err := os.Setenv("PATH", execPath+";"+os.Getenv("PATH")); err != nil {
		return err
	}

	if p.IsServer {
		return bootstrap.UpdateManifests(p.Resolver, p.IngressController, nodeConfig, cfg)
	}

	restConfig, err := clientcmd.BuildConfigFromFlags("", nodeConfig.AgentConfig.KubeConfigK3sController)

	p.CNIName, err = getCNIPluginName(restConfig)
	if err != nil {
		return err
	}

	// required to initialize KubeProxy
	p.KubeConfigKubeProxy = nodeConfig.AgentConfig.KubeConfigKubeProxy

	switch p.CNIName {
	case "", CNICalico:
		logrus.Info("Setting up Calico CNI")
		p.CNIPlugin = &win.Calico{}
	case CNIFlannel:
		logrus.Info("Setting up Flannel CNI")
		p.CNIPlugin = &win.Flannel{}
	case CNINone:
		logrus.Info("Skipping CNI setup")
		return nil
	default:
		logrus.Fatal("Unsupported CNI: ", p.CNIName)
	}

	if err := p.CNIPlugin.Setup(ctx, nodeConfig, restConfig, p.DataDir); err != nil {
		return err
	}

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

	// It should never happen but just in case, we make sure the rke2-uninstall.lock does not exist before starting kubelet
	lockFile := filepath.Join(p.DataDir, "bin", "rke2-uninstall.lock")
	if _, err := os.Stat(lockFile); err == nil {
		// If the file exists, delete it
		if err := os.Remove(lockFile); err != nil {
			logrus.Errorf("Failed to remove the %s file: %v", lockFile, err)
		}
	}

	win.ProcessWaitGroup.StartWithContext(ctx, func(ctx context.Context) {
		for {
			logrus.Infof("Running RKE2 kubelet %v", cleanArgs)
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

			// If the rke2-uninstall.ps1 script created the lock file, we are removing rke2 and thus we don't restart kubelet
			if _, err := os.Stat(lockFile); err == nil {
				logrus.Infof("rke2-uninstall.lock exists. kubelet is not restarted")
				return
			}

			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(5 * time.Second)
			}
		}
	})
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
		logrus.Errorf("Failed to reserve VIP for kube-proxy: %v", err)
	}
	logrus.Infof("Reserved VIP for kube-proxy: %s", vip)

	extraArgs := map[string]string{
		"network-name": CNIConfig.OverlayNetName,
		"bind-address": CNIConfig.NodeIP,
		"source-vip":   vip,
	}

	if err := hcn.DSRSupported(); err == nil {
		logrus.Infof("WinDSR support is enabled")
		extraArgs["feature-gates"] = addFeatureGate(extraArgs["feature-gates"], "WinDSR=true")
		extraArgs["enable-dsr"] = "true"
	}

	args = append(getArgs(extraArgs), args...)

	win.ProcessWaitGroup.StartWithContext(ctx, func(ctx context.Context) {
		outputFile := logging.GetLogger(filepath.Join(p.DataDir, "agent", "logs", "kube-proxy.log"), 50)
		for {
			logrus.Infof("Running RKE2 kube-proxy %s", args)
			cmd := exec.CommandContext(ctx, filepath.Join("c:\\", p.DataDir, "bin", "kube-proxy.exe"), args...)
			cmd.Stdout = outputFile
			cmd.Stderr = outputFile
			err := cmd.Run()
			logrus.Errorf("kube-proxy exited: %v", err)

			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(5 * time.Second)
			}
		}
	})

	return nil
}

// Docker starts cri-dockerd as implemented in the k3s cridockerd package
func (p *PEBinaryConfig) Docker(ctx context.Context, cfg *config.Node) error {
	return cridockerd.Run(ctx, cfg)
}

// Containerd configures and starts containerd.
func (p *PEBinaryConfig) Containerd(ctx context.Context, cfg *config.Node) error {
	args := getContainerdArgs(cfg)
	stdOut := io.Writer(os.Stdout)
	stdErr := io.Writer(os.Stderr)

	if cfg.Containerd.Log != "" {
		logrus.Infof("Logging containerd to %s", cfg.Containerd.Log)
		fileOut := &lumberjack.Logger{
			Filename:   cfg.Containerd.Log,
			MaxSize:    50,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		}

		// If rke2 is started with --debug, write logs to both the log file and stdout/stderr,
		// even if a log path is set.
		if cfg.Containerd.Debug {
			stdOut = io.MultiWriter(stdOut, fileOut)
			stdErr = io.MultiWriter(stdErr, fileOut)
		} else {
			stdOut = fileOut
			stdErr = fileOut
		}
	}

	win.ProcessWaitGroup.StartWithContext(ctx, func(ctx context.Context) {
		env := []string{}
		cenv := []string{}

		for _, e := range os.Environ() {
			pair := strings.SplitN(e, "=", 2)
			switch {
			case pair[0] == "NOTIFY_SOCKET":
				// elide NOTIFY_SOCKET to prevent spurious notifications to systemd
			case pair[0] == "CONTAINERD_LOG_LEVEL":
				// Turn CONTAINERD_LOG_LEVEL variable into log-level flag
				args = append(args, "--log-level", pair[1])
			case strings.HasPrefix(pair[0], "CONTAINERD_"):
				// Strip variables with CONTAINERD_ prefix before passing through
				// This allows doing things like setting a proxy for image pulls by setting
				// CONTAINERD_https_proxy=http://proxy.example.com:8080
				pair[0] = strings.TrimPrefix(pair[0], "CONTAINERD_")
				cenv = append(cenv, strings.Join(pair, "="))
			default:
				env = append(env, strings.Join(pair, "="))
			}
		}

		for {
			logrus.Infof("Running containerd %s", config.ArgString(args[1:]))
			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			cmd.Stdout = stdOut
			cmd.Stderr = stdErr
			cmd.Env = append(env, cenv...)

			if err := cmd.Run(); err != nil {
				logrus.Errorf("containerd exited: %v", err)
			}

			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(5 * time.Second)
			}
		}
	})

	if err := cri.WaitForService(ctx, cfg.Containerd.Address, "containerd"); err != nil {
		return err
	}

	return containerd.PreloadImages(ctx, cfg)
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

func getContainerdArgs(cfg *config.Node) []string {
	args := []string{
		"containerd",
		"-c", cfg.Containerd.Config,
	}
	return args
}

// getArgs converts a map to the correct args list format.
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
		case win.FlannelChart:
			return CNIFlannel, nil
		case "rke2-cilium":
			return CNICilium, nil
		case "rke2-canal":
			return CNICanal, nil
		}
	}
	return CNINone, nil
}

// setWindowsAgentSpecificSettings configures the correct paths needed for Windows
func setWindowsAgentSpecificSettings(dataDir string, nodeConfig *config.Node) {
	nodeConfig.AgentConfig.CNIBinDir = filepath.Join("c:\\", dataDir, "bin")
	nodeConfig.AgentConfig.CNIConfDir = filepath.Join("c:\\", dataDir, "agent", "etc", "cni")
}
