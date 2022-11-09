//go:build windows
// +build windows

package windows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/ghodss/yaml"
	wapi "github.com/iamacarpet/go-win64api"
	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	netroute "github.com/libp2p/go-netroute"
	"github.com/sirupsen/logrus"
	opv1 "github.com/tigera/operator/api/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

func (c *Calico) generateCalicoNetworks() error {
	if err := deleteAllNetworksOnNodeRestart(); err != nil {
		return err
	}

	mgmt, err := createHnsNetwork(c.CNICfg.Mode, os.Getenv("VXLAN_ADAPTER"))
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
		c.CNICfg.IPAutoDetectionMethod = IPAutoDetectionMethod
	}

	return nil
}

// nodeAddressAutodetection processes the HelmChartConfig info and returns the nodeAddressAutodetection method expected by Calico config
func nodeAddressAutodetection(autoDetect opv1.NodeAddressAutodetection) (string, error) {
	if autoDetect.FirstFound != nil && *autoDetect.FirstFound {
		return "first-found", nil
	}

	if autoDetect.CanReach != "" {
		return "can-reach=" + autoDetect.CanReach, nil
	}

	if autoDetect.Interface != "" {
		return "interface=" + autoDetect.Interface, nil
	}

	if autoDetect.SkipInterface != "" {
		return "skip-interface=" + autoDetect.SkipInterface, nil
	}

	if len(autoDetect.CIDRS) > 0 {
		return "cidr=" + strings.Join(autoDetect.CIDRS, ","), nil
	}

	return "", errors.New("the passed autoDetect value is not supported. Please read Calico docs")
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

	logrus.Infof("Felix Envs: ", append(generateGeneralCalicoEnvs(config), specificEnvs...))
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
	logrus.Infof("Calico Envs: ", append(generateGeneralCalicoEnvs(config), specificEnvs...))
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

func deleteAllNetworksOnNodeRestart() error {
	networks, err := hcsshim.HNSListNetworkRequest("GET", "", "")
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.Name != "nat" {
			_, err = network.Delete()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// createHnsNetwork creates the network that will connect nodes and returns its managementIP
func createHnsNetwork(backend string, vxlanAdapter string) (string, error) {
	var network hcsshim.HNSNetwork
	if backend == "vxlan" {
		// Ignoring the return because both true and false without an error represent that the firewall rule was created or already exists
		if _, err := wapi.FirewallRuleAdd("OverlayTraffic4789UDP", "Overlay network traffic UDP", "", "4789", wapi.NET_FW_IP_PROTOCOL_UDP, wapi.NET_FW_PROFILE2_ALL); err != nil {
			return "", fmt.Errorf("error creating firewall rules: %v", err)
		}
		logrus.Debugf("Creating VXLAN network using the vxlanAdapter: %s", vxlanAdapter)
		network = hcsshim.HNSNetwork{
			Type:               "Overlay",
			Name:               CalicoHnsNetworkName,
			NetworkAdapterName: vxlanAdapter,
			Subnets: []hcsshim.Subnet{
				{
					AddressPrefix:  "192.168.255.0/30",
					GatewayAddress: "192.168.255.1",
					Policies: []json.RawMessage{
						[]byte("{ \"Type\": \"VSID\", \"VSID\": 9999 }"),
					},
				},
			},
		}
	} else {
		return "", fmt.Errorf("The Calico backend %s is not supported. Only vxlan backend is supported", backend)
	}
	// Currently, only vxlan is supported. Leaving the code for future
	//} else {
	//	network = hcsshim.HNSNetwork{
	//		Type: "L2Bridge",
	//		Name: CalicoHnsNetworkName,
	//		Subnets: []hcsshim.Subnet{
	//			{
	//				AddressPrefix:  "192.168.255.0/30",
	//				GatewayAddress: "192.168.255.1",
	//			},
	//		},
	//	}
	//}

	if _, err := network.Create(); err != nil {
		return "", fmt.Errorf("error creating the %s network: %v", CalicoHnsNetworkName, err)
	}

	// Check if network exists. If it does not after 5 minutes, fail
	for start := time.Now(); time.Since(start) < 5*time.Minute; {
		network, err := hcsshim.GetHNSNetworkByName(CalicoHnsNetworkName)
		if err == nil {
			return network.ManagementIP, nil
		}
	}

	return "", fmt.Errorf("failed to create %s network", CalicoHnsNetworkName)
}

func platformType() (string, error) {
	aksNet, _ := hcsshim.GetHNSNetworkByName("azure")
	if aksNet != nil {
		return "aks", nil
	}

	eksNet, _ := hcsshim.GetHNSNetworkByName("vpcbr*")
	if eksNet != nil {
		return "eks", nil
	}

	// EC2
	ec2Resp, err := http.Get("http://169.254.169.254/latest/meta-data/local-hostname")
	if err != nil && hasTimedOut(err) {
		return "", err
	}
	if ec2Resp != nil {
		defer ec2Resp.Body.Close()
		if ec2Resp.StatusCode == http.StatusOK {
			return "ec2", nil
		}
	}

	// GCE
	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/hostname", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Metadata-Flavor", "Google")
	gceResp, err := client.Do(req)
	if err != nil && hasTimedOut(err) {
		return "", err
	}
	if gceResp != nil {
		defer gceResp.Body.Close()
		if gceResp.StatusCode == http.StatusOK {
			return "gce", nil
		}
	}

	return "bare-metal", nil
}

func hasTimedOut(err error) bool {
	switch err := err.(type) {
	case *url.Error:
		if err, ok := err.Err.(net.Error); ok && err.Timeout() {
			return true
		}
	case net.Error:
		if err.Timeout() {
			return true
		}
	case *net.OpError:
		if err.Timeout() {
			return true
		}
	}
	errTxt := "use of closed network connection"
	if err != nil && strings.Contains(err.Error(), errTxt) {
		return true
	}
	return false
}

func autoConfigureIpam(it string) bool {
	if it == "host-local" {
		return true
	}
	return false
}

func setMetaDataServerRoute(mgmt string) error {
	ip := net.ParseIP(mgmt)
	if ip == nil {
		return fmt.Errorf("not a valid ip")
	}

	metaIp := net.ParseIP("169.254.169.254/32")
	router, err := netroute.New()
	if err != nil {
		return err
	}

	_, _, preferredSrc, err := router.Route(ip)
	if err != nil {
		return err
	}

	_, _, _, err = router.RouteWithSrc(nil, preferredSrc, metaIp) // input not used on windows
	return err
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
