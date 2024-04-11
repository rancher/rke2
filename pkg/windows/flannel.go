//go:build windows
// +build windows

package windows

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Microsoft/hcsshim"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/rancher/rke2/pkg/logging"
	"github.com/sirupsen/logrus"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	hostlocaldisk "github.com/containernetworking/plugins/plugins/ipam/host-local/backend/disk"

	"k8s.io/utils/pointer"
)

var (
	flannelKubeConfigTemplate = template.Must(template.New("FlannelKubeconfig").Parse(`apiVersion: v1
kind: Config
clusters:
- name: kubernetes
  cluster:
    certificate-authority: {{ .KubeConfig.CertificateAuthority }}
    server: {{ .KubeConfig.Server }}
contexts:
- name: flannel@kubernetes
  context:
    cluster: kubernetes
    namespace: kube-system
    user: flannel
current-context: flannel@kubernetes
users:
- name: flannel
  user:
    token: {{ .KubeConfig.Token }}
`))

	// Flannel net-conf for flanneld
	flanneldConfigTemplate = template.Must(template.New("FlanneldCnfig").Funcs(replaceSlashWin).Parse(`{
"Network": "{{ .ClusterCIDR }}", 
"Backend": {
  "Type": "{{ .OverlayEncap }}",
  "VNI": {{ .VxlanVNI }},
  "Port": {{ .VxlanPort }}
  }
}`))

	flannelCniConflistTemplate = template.Must(template.New("FlannelCniConfig").Funcs(replaceSlashWin).Parse(`{
	"name":"flannel.4096",  
	"cniVersion":"{{ .CNIVersion }}",
	"plugins":[
	  {
		"type":"flannel",
		"capabilities": {
		  "portMappings": true,
		  "dns": true
		},
		"delegate": {
		  "type": "win-overlay",
		  "apiVersion": 2,
		  "Policies": [{
			  "Name": "EndpointPolicy",
			  "Value": {
				  "Type": "OutBoundNAT",
				  "Settings": {
					"Exceptions": [
					  "{{ .ClusterCIDR }}", "{{ .ServiceCIDR }}"
					]
				  }
			  }
		  }, {
			  "Name": "EndpointPolicy",
			  "Value": {
				  "Type": "SDNRoute",
				  "Settings": {
					"DestinationPrefix": "{{ .ServiceCIDR }}",
					"NeedEncap": true
				  }
			  }
		  }, {
			  "name": "EndpointPolicy",
			  "value": {
				  "Type": "ProviderAddress",
				  "Settings": {
					  "ProviderAddress": "{{ .NodeIP }}"
				  }
			  } 
		  }]
		}
	  }
	]
  }
`))
)

type Flannel struct {
	CNICfg     *FlannelConfig
	KubeClient *kubernetes.Clientset
}

type SourceVipResponse struct {
	CniVersion string `json:"cniVersion"`
	IPs        []struct {
		Address string `json:"address"`
		Gateway string `json:"gateway"`
	} `json:"ips"`
	DNS struct{} `json:"dns"`
}

const (
	flannelConfigName      = "07-flannel.conflist"
	flannelKubeConfigName  = "flannel.kubeconfig"
	flanneldConfigName     = "flanneld-net-conf.json"
	FlannelChart           = "rke2-flannel"
	hostlocalContainerID   = "kube-proxy"
	hostlocalInterfaceName = "source-vip"
	hostLocalDataDir       = "/var/lib/cni/networks"
)

// GetConfig returns the CNI configuration
func (f *Flannel) GetConfig() *CNICommonConfig {
	return &f.CNICfg.CNICommonConfig
}

// Setup creates the basic configuration required by the CNI.
func (f *Flannel) Setup(ctx context.Context, nodeConfig *daemonconfig.Node, restConfig *rest.Config, dataDir string) error {

	if err := f.initializeConfig(ctx, nodeConfig, restConfig, dataDir); err != nil {
		return err
	}

	if err := f.writeConfigFiles(); err != nil {
		return err
	}

	logrus.Info("Flannel required config files ready")
	return nil
}

// initializeConfig sets the default configuration in CNIConfig
func (f *Flannel) initializeConfig(ctx context.Context, nodeConfig *daemonconfig.Node, restConfig *rest.Config, dataDir string) error {
	var err error

	f.CNICfg = &FlannelConfig{
		CNICommonConfig: CNICommonConfig{
			Name:           "flannel",
			OverlayNetName: "flannel.4096",
			Hostname:       nodeConfig.AgentConfig.NodeName,
			ConfigPath:     filepath.Join("c:\\", dataDir, "agent"),
			OverlayEncap:   "vxlan",
			VxlanVNI:       "4096",
			VxlanPort:      "4789",
			ServiceCIDR:    nodeConfig.AgentConfig.ServiceCIDR.String(),
			ClusterCIDR:    nodeConfig.AgentConfig.ClusterCIDR.String(),
			CNIConfDir:     nodeConfig.AgentConfig.CNIConfDir,
			NodeIP:         nodeConfig.AgentConfig.NodeIP,
			CNIBinDir:      nodeConfig.AgentConfig.CNIBinDir,
			CNIVersion:     "1.0.0",
			Interface:      nodeConfig.AgentConfig.NodeIP,
		},
	}

	f.CNICfg.KubeConfig, f.KubeClient, err = f.createKubeConfigAndClient(ctx, restConfig)
	if err != nil {
		return err
	}

	logrus.Debugf("Flannel Config: %+v", f.CNICfg)

	return nil
}

// writeConfigFiles writes the three required files by Flannel
func (f *Flannel) writeConfigFiles() error {

	// Create flannelKubeConfig
	if err := f.renderFlannelConfig(filepath.Join(f.CNICfg.ConfigPath, flannelKubeConfigName), flannelKubeConfigTemplate); err != nil {
		return err
	}

	// Create flanneld config
	if err := f.renderFlannelConfig(filepath.Join(f.CNICfg.ConfigPath, flanneldConfigName), flanneldConfigTemplate); err != nil {
		return err
	}

	// Create flannel CNI conflist
	if err := f.renderFlannelConfig(filepath.Join(f.CNICfg.CNIConfDir, flannelConfigName), flannelCniConflistTemplate); err != nil {
		return err
	}

	return nil
}

// renderFlannelConfig creates the file and then renders the template using Flannel Config parameters
func (f *Flannel) renderFlannelConfig(path string, toRender *template.Template) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	output, err := os.Create(path)
	if err != nil {
		return err
	}

	defer output.Close()
	toRender.Execute(output, f.CNICfg)

	return nil
}

// createKubeConfigAndClient creates all needed for Flannel to contact kube-api
func (f *Flannel) createKubeConfigAndClient(ctx context.Context, restConfig *rest.Config) (*KubeConfig, *kubernetes.Clientset, error) {

	// Fill all information except for the token
	flannelKubeConfig := KubeConfig{
		Server:               "https://127.0.0.1:6443",
		CertificateAuthority: filepath.Join(f.CNICfg.ConfigPath, "server-ca.crt"),
	}

	// Generate the token request
	req := authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences:         []string{version.Program},
			ExpirationSeconds: pointer.Int64(60 * 60 * 24 * 365),
		},
	}

	// Register the token in the Flannel service account
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, err
	}
	serviceAccounts := client.CoreV1().ServiceAccounts("kube-system")
	token, err := serviceAccounts.CreateToken(ctx, "flannel", &req, metav1.CreateOptions{})
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create token for service account (kube-system/flannel)")
	}

	flannelKubeConfig.Token = token.Status.Token

	return &flannelKubeConfig, client, nil
}

// Start waits for the node to be ready and starts flannel
func (f *Flannel) Start(ctx context.Context) error {
	logPath := filepath.Join(f.CNICfg.ConfigPath, "logs", "flanneld.log")

	// Wait for the node to be registered in the cluster
	if err := wait.PollImmediateWithContext(ctx, 3*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		_, err := f.KubeClient.CoreV1().Nodes().Get(ctx, f.CNICfg.Hostname, metav1.GetOptions{})
		if err != nil {
			logrus.WithError(err).Warningf("Flanneld can't start because it can't find node, retrying %s", f.CNICfg.Hostname)
			return false, nil
		} else {
			logrus.Infof("Node %s registered. Flanneld can start", f.CNICfg.Hostname)
			return true, nil
		}
	}); err != nil {
		return err
	}

	go startFlannel(ctx, f.CNICfg, logPath)

	return nil
}

// startFlannel calls the flanneld binary with the correct config parameters and envs
func startFlannel(ctx context.Context, config *FlannelConfig, logPath string) {
	outputFile := logging.GetLogger(logPath, 50)

	specificEnvs := []string{
		fmt.Sprintf("NODE_NAME=%s", config.Hostname),
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}

	args := []string{
		fmt.Sprintf("--kubeconfig-file=%s", filepath.Join(config.ConfigPath, flannelKubeConfigName)),
		"--ip-masq",
		"--kube-subnet-mgr",
		"--iptables-forward-rules=false",
		fmt.Sprintf("--iface=%s", config.Interface),
		fmt.Sprintf("--net-config-path=%s", filepath.Join(config.ConfigPath, flanneldConfigName)),
	}

	logrus.Infof("Flanneld Envs: %s and args: %v", specificEnvs, args)
	cmd := exec.CommandContext(ctx, "flanneld.exe", args...)
	cmd.Env = append(specificEnvs)
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile
	if err := cmd.Run(); err != nil {
		logrus.Errorf("Flanneld has an error: %v. Check %s for extra information", err, logPath)
	}
	logrus.Error("Flanneld exited")
}

// ReserveSourceVip reserves an IP that will be used as source VIP by kube-proxy. It uses host-local CNI plugin to reserve the IP
func (f *Flannel) ReserveSourceVip(ctx context.Context) (string, error) {
	var network *hcsshim.HNSNetwork
	var err error

	logrus.Info("Reserving an IP on flannel HNS network for kube-proxy source vip")
	if err := wait.PollImmediateWithContext(ctx, 10*time.Second, 5*time.Minute, func(ctx context.Context) (bool, error) {
		network, err = hcsshim.GetHNSNetworkByName(f.CNICfg.OverlayNetName)
		if err != nil || network == nil {
			logrus.Debugf("can't find flannel HNS network, retrying %s", f.CNICfg.OverlayNetName)
			return false, nil
		}

		if network.ManagementIP == "" {
			logrus.Debugf("wait for flannel HNS network management IP, retrying %s", f.CNICfg.OverlayNetName)
			return false, nil
		}

		if network.ManagementIP != "" {
			logrus.Infof("Flannel HNS network ready with managementIP: %s", network.ManagementIP)
			return true, nil
		}
		return false, nil
	}); err != nil {
		return "", err
	}

	// Check if the source vip was already reserved using host-local library
	hostlocalStore, err := hostlocaldisk.New(f.CNICfg.OverlayNetName, hostLocalDataDir)
	if err != nil {
		return "", fmt.Errorf("failed to create host-local store: %w", err)
	}
	ips := hostlocalStore.GetByID(hostlocalContainerID, hostlocalInterfaceName)
	if len(ips) > 0 {
		logrus.Infof("Source VIP for kube-proxy was already reserved %v", ips)
		return strings.TrimSpace(strings.Split(ips[0].String(), "/")[0]), nil
	}

	logrus.Info("No source VIP for kube-proxy reserved. Creating one")
	subnet := network.Subnets[0].AddressPrefix

	logrus.Debugf("host-local will use the following subnet: %v to reserve the sourceIP", subnet)

	configData := `{
		"cniVersion": "1.0.0",
		"name": "` + f.CNICfg.OverlayNetName + `",
		"ipam": {
			"type": "host-local",
			"ranges": [[{"subnet":"` + subnet + `"}]],
			"dataDir": "` + hostLocalDataDir + `"
		}
	}`

	cmd := exec.Command("host-local.exe")
	cmd.Env = append(os.Environ(),
		"CNI_COMMAND=ADD",
		"CNI_CONTAINERID="+hostlocalContainerID,
		"CNI_NETNS=kube-proxy",
		"CNI_IFNAME="+hostlocalInterfaceName,
		"CNI_PATH="+f.CNICfg.CNIBinDir,
	)

	cmd.Stdin = strings.NewReader(configData)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logrus.WithError(err).Warning("Failed to execute host-local.exe")
		logrus.Infof("This is the output: %v", strings.TrimSpace(string(out)))
		return "", err
	}

	var sourceVipResp SourceVipResponse
	err = json.Unmarshal(out, &sourceVipResp)
	if err != nil {
		logrus.WithError(err).Warning("Failed to unmarshal sourceVip response")
		logrus.Infof("This is the error: %v", err)
		return "", err
	}

	if len(sourceVipResp.IPs) > 0 {
		return strings.TrimSpace(strings.Split(sourceVipResp.IPs[0].Address, "/")[0]), nil
	}

	return "", errors.New("no source vip reserved")
}
