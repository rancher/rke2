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
	"time"

	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	opv1 "github.com/tigera/operator/api/v1"
	authv1 "k8s.io/api/authentication/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

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
        {{- if eq .Mode "vxlan" }}
        "NeedEncap": true
	{{- else }}
        "NeedEncap": false
	{{- end }}
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
	calicoLogPath          = "C:\\var\\log\\"
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
		IP:                    nodeConfig.AgentConfig.NodeIP,
		IPAutoDetectionMethod: "first-found",
		Felix: FelixConfig{
			Metadataaddr: "none",
			Vxlanvni:     "4096",
			MacPrefix:    "0E-2A",
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
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	output, err := os.Create(path)
	if err != nil {
		return err
	}

	defer output.Close()
	toRender.Execute(output, c.CNICfg)

	return nil
}

// createKubeConfig creates all needed for Calico to contact kube-api
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
	if err := os.MkdirAll(calicoLogPath, 0755); err != nil {
		return fmt.Errorf("error creating %s directory: %v", calicoLogPath, err)
	}
	for {
		if err := startCalico(ctx, c.CNICfg); err != nil {
			time.Sleep(5 * time.Second)
			logrus.Errorf("Calico exited: %v. Retrying", err)
			continue
		}
		break
	}
	go startFelix(ctx, c.CNICfg)
	if c.CNICfg.Mode == "windows-bgp" {
		go startConfd(ctx, c.CNICfg)
	}

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
	networkAdapter := os.Getenv("VXLAN_ADAPTER")
	if networkAdapter == "" {
		if c.CNICfg.Interface != "" {
			networkAdapter = c.CNICfg.Interface
		}

		if c.CNICfg.Interface == "" && c.CNICfg.IP != "" {
			iFace, err := findInterface(c.CNICfg.IP)
			if err != nil {
				return err
			}
			networkAdapter = iFace
		}
	}

	mgmt, err := createHnsNetwork(c.CNICfg.Mode, networkAdapter)
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
		c.CNICfg.IPAutoDetectionMethod, c.CNICfg.Interface, err = findCalicoInterface(nodeV4)
		if err != nil {
			return err
		}
	}
	if bgpEnabled := overrides.Installation.CalicoNetwork.BGP; bgpEnabled != nil {
		if *bgpEnabled == opv1.BGPEnabled {
			c.CNICfg.Mode = "windows-bgp"
		}
	}
	return nil
}

func findCalicoInterface(nodeV4 *opv1.NodeAddressAutodetection) (IPAutoDetectionMethod, calicoInterface string, err error) {
	IPAutoDetectionMethod, err = nodeAddressAutodetection(*nodeV4)
	if err != nil {
		return "", "", err
	}

	if strings.Contains(IPAutoDetectionMethod, "cidrs") {
		calicoInterface, err = findInterfaceCIDR(nodeV4.CIDRS)
		if err != nil {
			return "", "", err
		}
	}

	if strings.Contains(IPAutoDetectionMethod, "interface") {
		calicoInterface, err = findInterfaceRegEx(nodeV4.Interface)
		if err != nil {
			return "", "", err
		}
	}
	if strings.Contains(IPAutoDetectionMethod, "can-reach") {
		calicoInterface, err = findInterfaceReach(nodeV4.CanReach)
		if err != nil {
			return "", "", err
		}
	}
	return
}

func startConfd(ctx context.Context, config *CalicoConfig) {
	outputFile, err := os.Create(calicoLogPath + "confd.log")
	if err != nil {
		logrus.Fatalf("error creating confd.log: %v", err)
		return
	}
	defer outputFile.Close()

	specificEnvs := []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}

	args := []string{
		"-confd",
		fmt.Sprintf("-confd-confdir=%s", filepath.Join(config.CNI.BinDir, "confd")),
	}

	logrus.Infof("Confd Envs: %s", append(generateGeneralCalicoEnvs(config), specificEnvs...))
	cmd := exec.CommandContext(ctx, "calico-node.exe", args...)
	cmd.Env = append(generateGeneralCalicoEnvs(config), specificEnvs...)
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile
	_ = os.Chdir(filepath.Join(config.CNI.BinDir, "confd"))
	_ = cmd.Run()
	logrus.Error("Confd exited")
}

func startFelix(ctx context.Context, config *CalicoConfig) {
	outputFile, err := os.Create(calicoLogPath + "felix.log")
	if err != nil {
		logrus.Fatalf("error creating felix.log: %v", err)
		return
	}
	defer outputFile.Close()

	specificEnvs := []string{
		fmt.Sprintf("FELIX_FELIXHOSTNAME=%s", config.Hostname),
		fmt.Sprintf("FELIX_VXLANVNI=%s", config.Felix.Vxlanvni),
		fmt.Sprintf("FELIX_DATASTORETYPE=%s", config.DatastoreType),
	}

	// Add OS variables related to Felix. As they come after, they'll overwrite the previous ones
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "FELIX_") {
			specificEnvs = append(specificEnvs, env)
		}
	}

	args := []string{
		"-felix",
	}

	logrus.Infof("Felix Envs: %s", append(generateGeneralCalicoEnvs(config), specificEnvs...))
	cmd := exec.CommandContext(ctx, "calico-node.exe", args...)
	cmd.Env = append(generateGeneralCalicoEnvs(config), specificEnvs...)
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile
	_ = cmd.Run()
	logrus.Error("Felix exited")
}

func startCalico(ctx context.Context, config *CalicoConfig) error {
	outputFile, err := os.Create(calicoLogPath + "calico-node.log")
	if err != nil {
		return fmt.Errorf("error creating calico-node.log: %v", err)
	}
	defer outputFile.Close()
	specificEnvs := []string{
		fmt.Sprintf("CALICO_NODENAME_FILE=%s", config.NodeNameFile),
		fmt.Sprintf("CALICO_NETWORKING_BACKEND=%s", config.Mode),
		fmt.Sprintf("CALICO_DATASTORE_TYPE=%s", config.DatastoreType),
		fmt.Sprintf("IP_AUTODETECTION_METHOD=%s", config.IPAutoDetectionMethod),
		fmt.Sprintf("VXLAN_VNI=%s", config.Felix.Vxlanvni),
	}

	// Add OS variables related to Calico. As they come after, they'll overwrite the previous ones
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "CALICO_") {
			specificEnvs = append(specificEnvs, env)
		}
	}

	args := []string{
		"-startup",
	}
	logrus.Infof("Calico Envs: %s", append(generateGeneralCalicoEnvs(config), specificEnvs...))
	cmd := exec.CommandContext(ctx, "calico-node.exe", args...)
	cmd.Env = append(generateGeneralCalicoEnvs(config), specificEnvs...)
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func generateGeneralCalicoEnvs(config *CalicoConfig) []string {
	return []string{
		fmt.Sprintf("KUBE_NETWORK=%s", config.KubeNetwork),
		fmt.Sprintf("KUBECONFIG=%s", config.KubeConfig.Path),
		fmt.Sprintf("NODENAME=%s", config.Hostname),
		fmt.Sprintf("CALICO_K8S_NODE_REF=%s", config.Hostname),

		fmt.Sprintf("IP=%s", config.IP),
		fmt.Sprintf("USE_POD_CIDR=%t", autoConfigureIpam(config.CNI.IpamType)),
	}
}
