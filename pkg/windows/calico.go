//go:build windows
// +build windows

package windows

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	authv1 "k8s.io/api/authentication/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/utils/pointer"
)

var (
	replaceSlashWin = template.FuncMap{
		"replace": func(s string) string {
			return strings.ReplaceAll(s, "\\", "\\\\")
		},
	}

	calicoKubeConfigTemplate = template.Must(template.New("CalicoKubeconfig").Parse(`apiVersion: v1
kind: Config
clusters:
- name: kubernetes
  cluster:
    certificate-authority: {{ .KubeConfig.CertificateAuthority }}
    server: {{ .KubeConfig.Server }}
contexts:
- name: calico-windows@kubernetes
  context:
    cluster: kubernetes
    namespace: kube-system
    user: calico-windows
current-context: calico-windows@kubernetes
users:
- name: calico-windows
  user:
    token: {{ .KubeConfig.Token }}
`))

	// Config following definition from https://github.com/projectcalico/calico/blob/master/cni-plugin/pkg/types/types.go#L65-L131
	calicoConfigTemplate = template.Must(template.New("CalicoConfig").Funcs(replaceSlashWin).Parse(`{
  "name": "{{ .Name }}",
  "windows_use_single_network": true,
  "cniVersion": "{{ .CNI.Version }}",
  "type": "calico",
  "mode": "{{ .Mode }}",
  "vxlan_mac_prefix":  "{{ .Felix.MacPrefix }}",
  "vxlan_vni": {{ .Felix.Vxlanvni }},
  "policy": {
    "type": "k8s"
  },
  "log_level": "info",
  "capabilities": {"dns": true},
  "DNS": {
    "Nameservers": [
		"{{ .DNSServers }}"
    ],
    "Search":  [
      "svc.cluster.local"
    ]
  },
  "nodename_file": "{{ replace .NodeNameFile }}",
  "datastore_type": "{{ .DatastoreType }}",
  "etcd_endpoints": "{{ .ETCDEndpoints }}",
  "etcd_key_file": "{{ .ETCDKeyFile }}",
  "etcd_cert_file": "{{ .ETCDCertFile }}",
  "etcd_ca_cert_file": "{{ .ETCDCaCertFile }}",
  "kubernetes": {
    "kubeconfig": "{{ replace .KubeConfig.Path }}"
  },
  "ipam": {
    "type": "{{ .CNI.IpamType }}",
    "subnet": "usePodCidr"
  },
  "policies":  [
    {
      "Name":  "EndpointPolicy",
      "Value":  {
        "Type":  "OutBoundNAT",
        "ExceptionList":  [
          "{{ .ServiceCIDR }}"
        ]
      }
    },
    {
      "Name":  "EndpointPolicy",
      "Value":  {
        "Type":  "SDNROUTE",
        "DestinationPrefix":  "{{ .ServiceCIDR }}",
        "NeedEncap":  true
      }
    }
  ]
}
`))
)

type Calico struct {
	CNICfg  *CalicoConfig
	DataDir string
}

const (
	CalicoConfigName       = "10-calico.conf"
	CalicoKubeConfigName   = "calico.kubeconfig"
	CalicoNodeNameFileName = "calico_node_name"
	CalicoHnsNetworkName   = "External"
	CalicoSystemNamespace  = "calico-system"
	CalicoChart            = "rke2-calico"
	calicoNode             = "calico-node"
)

// Setup creates the basic configuration required by the CNI.
func (c *Calico) Setup(ctx context.Context, nodeConfig *daemonconfig.Node, restConfig *rest.Config, dataDir string) error {
	c.DataDir = dataDir

	if err := c.initializeConfig(ctx, nodeConfig, restConfig); err != nil {
		return err
	}

	if err := c.overrideCalicoConfigByHelm(restConfig); err != nil {
		return err
	}

	if err := c.writeConfigFiles(nodeConfig.AgentConfig.CNIConfDir, nodeConfig.AgentConfig.NodeName); err != nil {
		return err
	}

	logrus.Info("Generating HNS networks, please wait")
	return c.generateCalicoNetworks()
}

// initializeConfig sets the default configuration in CNIConfig
func (c *Calico) initializeConfig(ctx context.Context, nodeConfig *daemonconfig.Node, restConfig *rest.Config) error {
	platformType, err := platformType()
	if err != nil {
		return err
	}

	c.CNICfg = &CalicoConfig{
		Name:                  "Calico",
		OverlayNetName:        "Calico",
		Hostname:              nodeConfig.AgentConfig.NodeName,
		NodeNameFile:          filepath.Join("c:\\", c.DataDir, "agent", CalicoNodeNameFileName),
		KubeNetwork:           "Calico.*",
		Mode:                  "vxlan",
		ServiceCIDR:           nodeConfig.AgentConfig.ServiceCIDR.String(),
		DNSServers:            nodeConfig.AgentConfig.ClusterDNS.String(),
		DNSSearch:             "svc." + nodeConfig.AgentConfig.ClusterDomain,
		DatastoreType:         "kubernetes",
		Platform:              platformType,
		StartUpValidIPTimeout: 90,
		IP:                    nodeConfig.AgentConfig.NodeIP,
		IPAutoDetectionMethod: "first-found",
		Felix: FelixConfig{
			Metadataaddr:    "none",
			Vxlanvni:        "4096",
			MacPrefix:       "0E-2A",
			LogSeverityFile: "none",
			LogSeveritySys:  "none",
		},
		CNI: CalicoCNIConfig{
			BinDir:   nodeConfig.AgentConfig.CNIBinDir,
			ConfDir:  nodeConfig.AgentConfig.CNIConfDir,
			IpamType: "calico-ipam",
			Version:  "0.3.1",
		},
	}

	c.CNICfg.KubeConfig, err = c.createKubeConfig(ctx, restConfig)
	if err != nil {
		return err
	}

	return nil
}

// writeConfigFiles writes the three required files by Calico
func (c *Calico) writeConfigFiles(CNIConfDir string, NodeName string) error {

	// Create CalicoKubeConfig and CIPAutoDetectionMethodalicoConfig files
	if err := c.renderCalicoConfig(c.CNICfg.KubeConfig.Path, calicoKubeConfigTemplate); err != nil {
		return err
	}

	if err := c.renderCalicoConfig(filepath.Join(CNIConfDir, CalicoConfigName), calicoConfigTemplate); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join("c:\\", c.DataDir, "agent", CalicoNodeNameFileName), []byte(NodeName), 0644)
}

// renderCalicoConfig creates the file and then renders the template using Calico Config parameters
func (c *Calico) renderCalicoConfig(path string, toRender *template.Template) error {
	os.MkdirAll(filepath.Dir(path), 0755)
	output, err := os.Create(path)
	if err != nil {
		return err
	}

	defer output.Close()
	toRender.Execute(output, c.CNICfg)

	return nil
}

// createCalicoKubeConfig creates all needed for Calico to contact kube-api
func (c *Calico) createKubeConfig(ctx context.Context, restConfig *rest.Config) (*CalicoKubeConfig, error) {

	// Fill all information except for the token
	calicoKubeConfig := CalicoKubeConfig{
		Server:               "https://127.0.0.1:6443",
		CertificateAuthority: filepath.Join("c:\\", c.DataDir, "agent", "server-ca.crt"),
		Path:                 filepath.Join("c:\\", c.DataDir, "agent", CalicoKubeConfigName),
	}

	// Generate the token request
	req := authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences:         []string{version.Program},
			ExpirationSeconds: pointer.Int64(60 * 60 * 24 * 365),
		},
	}

	// Register the token in the Calico service account
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	serviceAccounts := client.CoreV1().ServiceAccounts(CalicoSystemNamespace)
	token, err := serviceAccounts.CreateToken(ctx, calicoNode, &req, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create token for service account (%s/%s)", CalicoSystemNamespace, calicoNode)
	}

	calicoKubeConfig.Token = token.Status.Token

	return &calicoKubeConfig, nil
}

// Start starts the CNI services on the Windows node.
func (c *Calico) Start(ctx context.Context) error {
	for {
		if err := startCalico(ctx, c.CNICfg); err != nil {
			continue
		}
		break
	}
	go startFelix(ctx, c.CNICfg)

	return nil
}

// generateCalicoNetworks creates the overlay networks for internode networking
func (c *Calico) generateCalicoNetworks() error {
	if err := deleteAllNetworks(); err != nil {
		return err
	}

	// There are four ways to select the vxlan interface. In order of priority:
	// 1 - VXLAN_ADAPTER env variable
	// 2 - c.CNICfg.Interface which set if NodeAddressAutodetection is set (Calico HelmChart)
	// 3 - nodeIP if defined
	// 4 - None of the above. In that case, by default the interface with the default route is picked
	vxlanAdapter := os.Getenv("VXLAN_ADAPTER")
	if vxlanAdapter == "" {
		if c.CNICfg.Interface != "" {
			vxlanAdapter = c.CNICfg.Interface
		}

		if c.CNICfg.Interface == "" && c.CNICfg.IP != "" {
			iFace, err := findInterface(c.CNICfg.IP)
			if err != nil {
				return err
			}
			vxlanAdapter = iFace
		}
	}

	mgmt, err := createHnsNetwork(c.CNICfg.Mode, vxlanAdapter)
	if err != nil {
		return err
	}

	if c.CNICfg.Platform == "ec2" || c.CNICfg.Platform == "gce" {
		logrus.Debugf("recreating metadata route because platform is: %s", c.CNICfg.Platform)
		if err := setMetaDataServerRoute(mgmt); err != nil {
			return err
		}
	}
	return nil
}

// overrideCalicoConfigByHelm overrides the default values set for calico if a Chart exists
func (c *Calico) overrideCalicoConfigByHelm(restConfig *rest.Config) error {
	hc, err := helm.NewFactoryFromConfig(restConfig)
	if err != nil {
		return err
	}

	cniChartConfig, err := hc.Helm().V1().HelmChartConfig().Get(metav1.NamespaceSystem, CalicoChart, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to check for %s HelmChartConfig", CalicoChart)
	}
	if cniChartConfig == nil {
		logrus.Debug("no CNI related HelmChartConfig found")
		return nil
	}
	overrides := CalicoInstallation{}
	if err := yaml.Unmarshal([]byte(cniChartConfig.Spec.ValuesContent), &overrides); err != nil {
		return err
	}
	// Marshal for clean debug logs, otherwise it's all pointers
	b, err := yaml.Marshal(overrides)
	if err != nil {
		return err
	}
	logrus.Debugf("calico override found: %s\n", string(b))
	if nodeV4 := overrides.Installation.CalicoNetwork.NodeAddressAutodetectionV4; nodeV4 != nil {
		IPAutoDetectionMethod, err := nodeAddressAutodetection(*nodeV4)
		if err != nil {
			return err
		}
		logrus.Debugf("this is IPAutoDetectionMethod: %s", IPAutoDetectionMethod)
		c.CNICfg.IPAutoDetectionMethod = IPAutoDetectionMethod

		var calicoInterface string
		if strings.Contains(IPAutoDetectionMethod, "cidrs") {
			calicoInterface, err = findInterfaceCIDR(nodeV4.CIDRS)
			if err != nil {
				return err
			}
		}

		if strings.Contains(IPAutoDetectionMethod, "interface") {
			calicoInterface, err = findInterfaceRegEx(nodeV4.Interface)
			if err != nil {
				return err
			}
		}

		if strings.Contains(IPAutoDetectionMethod, "can-reach") {
			calicoInterface, err = findInterfaceReach(nodeV4.CanReach)
			if err != nil {
				return err
			}
		}

		c.CNICfg.Interface = calicoInterface
	}

	return nil
}

func startFelix(ctx context.Context, config *CalicoConfig) {
	specificEnvs := []string{
		fmt.Sprintf("FELIX_FELIXHOSTNAME=%s", config.Hostname),
		fmt.Sprintf("FELIX_VXLANVNI=%s", config.Felix.Vxlanvni),
		fmt.Sprintf("FELIX_METADATAADDR=%s", config.Felix.Metadataaddr),
	}

	args := []string{
		"-felix",
	}

	logrus.Infof("Felix Envs: %s", append(generateGeneralCalicoEnvs(config), specificEnvs...))
	cmd := exec.CommandContext(ctx, "calico-node.exe", args...)
	cmd.Env = append(generateGeneralCalicoEnvs(config), specificEnvs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("Felix exited: %v", err)
	}
}

func startCalico(ctx context.Context, config *CalicoConfig) error {
	specificEnvs := []string{
		fmt.Sprintf("CALICO_NODENAME_FILE=%s", config.NodeNameFile),
	}

	args := []string{
		"-startup",
	}
	logrus.Infof("Calico Envs: %s", append(generateGeneralCalicoEnvs(config), specificEnvs...))
	cmd := exec.CommandContext(ctx, "calico-node.exe", args...)
	cmd.Env = append(generateGeneralCalicoEnvs(config), specificEnvs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		logrus.Errorf("Calico exited: %v", err)
		return err
	}
	return nil
}

func generateGeneralCalicoEnvs(config *CalicoConfig) []string {
	return []string{
		fmt.Sprintf("KUBE_NETWORK=%s", config.KubeNetwork),
		fmt.Sprintf("KUBECONFIG=%s", config.KubeConfig.Path),
		fmt.Sprintf("K8S_SERVICE_CIDR=%s", config.ServiceCIDR),
		fmt.Sprintf("NODENAME=%s", config.Hostname),

		fmt.Sprintf("CALICO_NETWORKING_BACKEND=%s", config.Mode),
		fmt.Sprintf("CALICO_DATASTORE_TYPE=%s", config.DatastoreType),
		fmt.Sprintf("CALICO_K8S_NODE_REF=%s", config.Hostname),
		fmt.Sprintf("CALICO_LOG_DIR=%s", config.LogDir),

		fmt.Sprintf("DNS_NAME_SERVERS=%s", config.DNSServers),
		fmt.Sprintf("DNS_SEARCH=%s", config.DNSSearch),

		fmt.Sprintf("ETCD_ENDPOINTS=%s", config.ETCDEndpoints),
		fmt.Sprintf("ETCD_KEY_FILE=%s", config.ETCDKeyFile),
		fmt.Sprintf("ETCD_CERT_FILE=%s", config.ETCDCertFile),
		fmt.Sprintf("ETCD_CA_CERT_FILE=%s", config.ETCDCaCertFile),

		fmt.Sprintf("CNI_BIN_DIR=%s", config.CNI.BinDir),
		fmt.Sprintf("CNI_CONF_DIR=%s", config.CNI.ConfDir),
		fmt.Sprintf("CNI_CONF_FILENAME=%s", config.CNI.ConfFileName),
		fmt.Sprintf("CNI_IPAM_TYPE=%s", config.CNI.IpamType),

		fmt.Sprintf("FELIX_LOGSEVERITYFILE=%s", config.Felix.LogSeverityFile),
		fmt.Sprintf("FELIX_LOGSEVERITYSYS=%s", config.Felix.LogSeveritySys),

		fmt.Sprintf("STARTUP_VALID_IP_TIMEOUT=90"),
		fmt.Sprintf("IP=%s", config.IP),
		fmt.Sprintf("IP_AUTODETECTION_METHOD=%s", config.IPAutoDetectionMethod),

		fmt.Sprintf("USE_POD_CIDR=%t", autoConfigureIpam(config.CNI.IpamType)),

		fmt.Sprintf("VXLAN_VNI=%s", config.Felix.Vxlanvni),
	}
}
