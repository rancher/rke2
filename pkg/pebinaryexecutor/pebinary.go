// +build windows

package pebinaryexecutor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/Microsoft/hcsshim/hcn"
	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	"github.com/rancher/k3s/pkg/cli/cmds"
	daemonconfig "github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/rancher/rke2/pkg/bootstrap"
	"github.com/rancher/rke2/pkg/images"
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
	ManifestsDir    string
	ImagesDir       string
	Resolver        *images.Resolver
	CloudProvider   *CloudProviderConfig
	CISMode         bool
	DataDir         string
	AuditPolicyFile string
	KubeletPath     string
	DisableETCD     bool
	IsServer        bool
	cniConig        *win.CNIConfig
	cni             win.CNI
}

type CloudProviderConfig struct {
	Name string
	Path string
}

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
	cniType, err := getCniType(restConfig)
	if err != nil {
		return err
	}

	switch cniType {
	case "calico":
		p.cni = &win.Calico{}
	default:
		return fmt.Errorf("the CNI %s isn't supported on Windows", cniType)
	}

	cniConfig, err := p.cni.Setup(ctx, p.DataDir, nodeConfig, restConfig)
	if err != nil {
		return err
	}
	p.cniConig = cniConfig

	logrus.Infof("Okay, exiting setup.")
	return nil
}

// Kubelet starts the kubelet in a subprocess with watching goroutine.
func (p *PEBinaryConfig) Kubelet(args []string) error {
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
			cmd := exec.Command(p.KubeletPath, cleanArgs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			logrus.Errorf("Kubelet exited: %v", err)
			time.Sleep(5 * time.Second)
		}
	}()
	return p.cni.Start(p.cniConig)
}

// KubeProxy starts the kubeproxy in a subprocess with watching goroutine.
func (p *PEBinaryConfig) KubeProxy(args []string) error {
	extraArgs := map[string]string{
		"hostname-override": p.cniConig.CalicoConfig.Hostname,
		"v":                 "4",
		"proxy-mode":        "kernelspace",
		"kubeconfig":        p.cniConig.NodeConfig.AgentConfig.KubeConfigKubeProxy,
		"network-name":      p.cniConig.NetworkName,
		"bind-address":      p.cniConig.BindAddress,
	}

	if err := hcn.DSRSupported(); err == nil {
		logrus.Infof("WinDSR support is enabled")
		extraArgs["feature-gates"] = addFeatureGate(extraArgs["feature-gates"], "WinDSR=true")
		extraArgs["enable-dsr"] = "true"
	}

	for range time.Tick(time.Second * 5) {
		endpoint, err := hcsshim.GetHNSEndpointByName("Calico_ep")
		if err != nil {
			logrus.WithError(err).Warningf("can't find %s, retrying", "Calico_ep")
			continue
		}
		extraArgs["source-vip"] = endpoint.IPAddress.String()
		break
	}

	logrus.Infof("Deleting HNS policies before kube-proxy starts.")
	policies, _ := hcsshim.HNSListPolicyListRequest()
	for _, policy := range policies {
		policy.Delete()
	}

	args = append(getArgs(extraArgs), args...)
	logrus.Infof("Running RKE2 kube-proxy %s", args)
	go func() {
		for {
			cmd := exec.Command(filepath.Join("c:\\", p.DataDir, "bin", "kube-proxy.exe"), args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			logrus.Errorf("kube-proxy exited: %v", err)
			time.Sleep(5 * time.Second)
		}
	}()

	return nil
}

// APIServer isn't supported in the binary executor.
func (p *PEBinaryConfig) APIServer(ctx context.Context, etcdReady <-chan struct{}, args []string) (authenticator.Request, http.Handler, error) {
	panic("kube-api-server is unsupported on windows")
}

// Scheduler isn't supported in the binary executor.
func (p *PEBinaryConfig) Scheduler(apiReady <-chan struct{}, args []string) error {
	panic("kube-scheduler is unsupported on windows")
}

// ControllerManager isn't supported in the binary executor.
func (p *PEBinaryConfig) ControllerManager(apiReady <-chan struct{}, args []string) error {
	panic("kube-controller-manager is unsupported on windows")
}

// CurrentETCDOptions isn't supported in the binary executor.
func (p *PEBinaryConfig) CurrentETCDOptions() (opts executor.InitialOptions, err error) {
	panic("etcd options are unsupported on windows")
}

// CloudControllerManager isn't supported in the binary executor.
func (p *PEBinaryConfig) CloudControllerManager(ccmRBACReady <-chan struct{}, args []string) error {
	panic("cloud-controller-manager is unsupported on windows.")
}

// ETCD isn't supported in the binary executor.
func (p *PEBinaryConfig) ETCD(args executor.ETCDConfig) error {
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

func getCniType(restConfig *rest.Config) (string, error) {
	hc, err := helm.NewFactoryFromConfig(restConfig)
	if err != nil {
		return "", err
	}
	hl, err := hc.Helm().V1().HelmChart().List(metav1.NamespaceSystem, metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, h := range hl.Items {
		if h.Name == win.CalicoChart {
			return "calico", nil
		}
	}
	return "", errors.New("calico chart was not found")
}

// setWindowsAgentSpecificSettings configures the correct paths needed for Windows
func setWindowsAgentSpecificSettings(dataDir string, nodeConfig *daemonconfig.Node) {
	nodeConfig.AgentConfig.CNIBinDir = filepath.Join("c:\\", dataDir, "bin")
	nodeConfig.AgentConfig.CNIConfDir = filepath.Join("c:\\", dataDir, "agent", "etc", "cni")
}
