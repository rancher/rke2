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

	"github.com/Microsoft/hcsshim"
	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	pkgerrors "github.com/pkg/errors"
	"github.com/rancher/rke2/pkg/logging"
	"github.com/sirupsen/logrus"
	opv1 "github.com/tigera/operator/api/v1"
	"golang.org/x/sys/windows"
	authv1 "k8s.io/api/authentication/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"

	"k8s.io/utils/pointer"
)

var (
	calicoKubeConfigTemplate = template.Must(template.New("Kubeconfig").Parse(`apiVersion: v1
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
  "cniVersion": "{{ .CNIVersion }}",
  "type": "calico",
  "mode": "{{ .OverlayEncap }}",
  "vxlan_mac_prefix": "0E-2A",
  "vxlan_vni": {{ .VxlanVNI }},
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
    "type": "{{ .IpamType }}",
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
        {{- if eq .OverlayEncap "vxlan" }}
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

// explicit interface check
var _ CNIPlugin = &Calico{}

type Calico struct {
	CNICfg     *CalicoConfig
	KubeClient *kubernetes.Clientset
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

// GetConfig returns the CNI configuration
func (c *Calico) GetConfig() *CNICommonConfig {
	return &c.CNICfg.CNICommonConfig
}

// Setup creates the basic configuration required by the CNI.
func (c *Calico) Setup(ctx context.Context, nodeConfig *daemonconfig.Node, restConfig *rest.Config, dataDir string) error {
	if err := c.initializeConfig(ctx, nodeConfig, restConfig, dataDir); err != nil {
		return err
	}

	if err := c.overrideCalicoConfigByHelm(restConfig); err != nil {
		return err
	}

	return c.writeConfigFiles()
}

// initializeConfig sets the default configuration in CNIConfig
func (c *Calico) initializeConfig(ctx context.Context, nodeConfig *daemonconfig.Node, restConfig *rest.Config, dataDir string) error {
	platformType, err := platformType()
	if err != nil {
		return err
	}

	c.CNICfg = &CalicoConfig{
		CNICommonConfig: CNICommonConfig{
			Name:           "Calico",
			OverlayNetName: "Calico",
			OverlayEncap:   "vxlan",
			Hostname:       nodeConfig.AgentConfig.NodeName,
			ConfigPath:     filepath.Join("c:\\", dataDir, "agent"),
			CNIConfDir:     nodeConfig.AgentConfig.CNIConfDir,
			CNIBinDir:      nodeConfig.AgentConfig.CNIBinDir,
			ClusterCIDR:    nodeConfig.AgentConfig.ClusterCIDR.String(),
			ServiceCIDR:    nodeConfig.AgentConfig.ServiceCIDR.String(),
			NodeIP:         nodeConfig.AgentConfig.NodeIP,
			VxlanVNI:       "4096",
			VxlanPort:      "4789",
			IpamType:       "calico-ipam",
			CNIVersion:     "0.3.1",
		},
		NodeNameFile:          filepath.Join("c:\\", dataDir, "agent", CalicoNodeNameFileName),
		KubeNetwork:           "Calico.*",
		DNSServers:            nodeConfig.AgentConfig.ClusterDNS.String(),
		DNSSearch:             "svc." + nodeConfig.AgentConfig.ClusterDomain,
		DatastoreType:         "kubernetes",
		Platform:              platformType,
		IPAutoDetectionMethod: "first-found",
	}

	c.CNICfg.KubeConfig, c.KubeClient, err = c.createKubeConfigAndClient(ctx, restConfig)
	if err != nil {
		return err
	}

	logrus.Debugf("Calico Config: %+v", c.CNICfg)

	return nil
}

// writeConfigFiles writes the three required files by Calico
func (c *Calico) writeConfigFiles() error {

	// Create CalicoKubeConfig and CIPAutoDetectionMethodalicoConfig files
	if err := c.renderCalicoConfig(c.CNICfg.KubeConfig.Path, calicoKubeConfigTemplate); err != nil {
		return err
	}

	if err := c.renderCalicoConfig(filepath.Join(c.CNICfg.CNIConfDir, CalicoConfigName), calicoConfigTemplate); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(c.CNICfg.ConfigPath, CalicoNodeNameFileName), []byte(c.CNICfg.Hostname), 0644)
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

// createKubeConfigAndClient creates all needed for Calico to contact kube-api
func (c *Calico) createKubeConfigAndClient(ctx context.Context, restConfig *rest.Config) (*KubeConfig, *kubernetes.Clientset, error) {

	// Fill all information except for the token
	calicoKubeConfig := KubeConfig{
		Server:               "https://127.0.0.1:6443",
		CertificateAuthority: filepath.Join(c.CNICfg.ConfigPath, "server-ca.crt"),
		Path:                 filepath.Join(c.CNICfg.ConfigPath, CalicoKubeConfigName),
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
		return nil, nil, err
	}
	serviceAccounts := client.CoreV1().ServiceAccounts(CalicoSystemNamespace)
	token, err := serviceAccounts.CreateToken(ctx, calicoNode, &req, metav1.CreateOptions{})
	if err != nil {
		return nil, nil, pkgerrors.WithMessagef(err, "failed to create token for service account (%s/%s)", CalicoSystemNamespace, calicoNode)
	}

	calicoKubeConfig.Token = token.Status.Token

	return &calicoKubeConfig, client, nil
}

// Start starts the CNI services on the Windows node.
func (c *Calico) Start(ctx context.Context) error {
	logPath := filepath.Join(c.CNICfg.ConfigPath, "logs")

	logrus.Info("Generating HNS networks, please wait")
	if err := c.generateCalicoNetworks(); err != nil {
		return err
	}

	// Wait for the node to be registered in the cluster
	if err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := c.KubeClient.CoreV1().Nodes().Get(ctx, c.CNICfg.Hostname, metav1.GetOptions{})
		if err != nil {
			logrus.WithError(err).Warningf("Calico can't start because it can't find node %s, retrying...", c.CNICfg.Hostname)
			return false, nil
		}

		logrus.Infof("Node %s registered. Calico can start", c.CNICfg.Hostname)

		if err := startCalico(ctx, c.CNICfg, logPath); err != nil {
			logrus.Errorf("Calico startup failed: %v, retrying...", err)
			return false, nil
		}
		return true, nil
	}); err != nil {
		return err
	}

	go runFelix(ctx, c.CNICfg, logPath)
	go runConfd(ctx, c.CNICfg, logPath)

	// Delete policies in case calico network is being reused
	policies, _ := hcsshim.HNSListPolicyListRequest()
	for _, policy := range policies {
		policy.Delete()
	}

	return reserveCalicoVIP(ctx, c.CNICfg)
}

// generateCalicoNetworks creates the overlay networks for internode networking
func (c *Calico) generateCalicoNetworks() error {
	nodeRebooted, err := c.isNodeRebooted()
	if err != nil {
		return pkgerrors.WithMessagef(err, "failed to check last node reboot time")
	}
	if nodeRebooted {
		if err = deleteAllNetworks(); err != nil {
			return pkgerrors.WithMessagef(err, "failed to delete all networks before bootstrapping calico")
		}
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

		if c.CNICfg.Interface == "" && c.CNICfg.NodeIP != "" {
			iFace, err := findInterface(c.CNICfg.NodeIP)
			if err != nil {
				return err
			}
			networkAdapter = iFace
		}
	}

	mgmt, err := createHnsNetwork(c.CNICfg.OverlayEncap, networkAdapter)
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
			c.CNICfg.OverlayEncap = "windows-bgp"
		}
	}
	return nil
}

// findCalicoInterface finds the interface to use for Calico based on the NodeAddressAutodetectionV4
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

// runConfd starts the confd service (for BGP)
func runConfd(ctx context.Context, config *CalicoConfig, logPath string) {
	logPath = filepath.Join(logPath, "confd.log")
	if config.OverlayEncap != "windows-bgp" {
		return
	}

	specificEnvs := []string{
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}

	args := []string{
		"-confd",
		fmt.Sprintf("-confd-confdir=%s", filepath.Join(config.CNIBinDir, "confd")),
	}
	logrus.Infof("Confd Envs: %s", append(generateGeneralCalicoEnvs(config), specificEnvs...))

	for {
		logrus.Infof("Confd logging to %s", logPath)
		outputFile := logging.GetLogger(logPath, 50)
		cmd := exec.CommandContext(ctx, "calico-node.exe", args...)
		cmd.Env = append(generateGeneralCalicoEnvs(config), specificEnvs...)
		cmd.Stdout = outputFile
		cmd.Stderr = outputFile
		os.Chdir(filepath.Join(config.CNIBinDir, "confd"))
		err := cmd.Run()
		if ctx.Err() != nil {
			return
		}
		if eerr, ok := err.(*exec.ExitError); ok && eerr.ProcessState.ExitCode() == 129 {
			logrus.Infof("Confd exited for config reload; restarting...")
		}
		logrus.WithError(err).Info("Confd exited, restarting...")
		outputFile.Close()
		time.Sleep(time.Second)
	}
}

// runFelix starts the felix service
func runFelix(ctx context.Context, config *CalicoConfig, logPath string) {
	logPath = filepath.Join(logPath, "felix.log")
	specificEnvs := []string{
		fmt.Sprintf("FELIX_FELIXHOSTNAME=%s", config.Hostname),
		fmt.Sprintf("FELIX_VXLANVNI=%s", config.VxlanVNI),
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

	for {
		logrus.Infof("Felix logging to %s", logPath)
		outputFile := logging.GetLogger(logPath, 50)
		cmd := exec.CommandContext(ctx, "calico-node.exe", args...)
		cmd.Env = append(generateGeneralCalicoEnvs(config), specificEnvs...)
		cmd.Stdout = outputFile
		cmd.Stderr = outputFile
		err := cmd.Run()
		if ctx.Err() != nil {
			return
		}
		if eerr, ok := err.(*exec.ExitError); ok && eerr.ProcessState.ExitCode() == 129 {
			logrus.Infof("Felix exited for config reload; restarting...")
		}
		logrus.WithError(err).Info("Felix exited, restarting...")
		outputFile.Close()
	}
}

// startCalico runs the calico non-privileged start-up routine.
// This is expected to exit with 0 exit code when the CNI is ready to operate.
func startCalico(ctx context.Context, config *CalicoConfig, logPath string) error {
	logPath = filepath.Join(logPath, "calico-node.log")
	outputFile := logging.GetLogger(logPath, 50)
	defer outputFile.Close()

	specificEnvs := []string{
		fmt.Sprintf("CALICO_NODENAME_FILE=%s", config.NodeNameFile),
		fmt.Sprintf("CALICO_NETWORKING_BACKEND=%s", config.OverlayEncap),
		fmt.Sprintf("CALICO_DATASTORE_TYPE=%s", config.DatastoreType),
		fmt.Sprintf("IP_AUTODETECTION_METHOD=%s", config.IPAutoDetectionMethod),
		fmt.Sprintf("VXLAN_VNI=%s", config.VxlanVNI),
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
	logrus.Infof("Calico logging to %s", logPath)
	cmd := exec.CommandContext(ctx, "calico-node.exe", args...)
	cmd.Env = append(generateGeneralCalicoEnvs(config), specificEnvs...)
	cmd.Stdout = outputFile
	cmd.Stderr = outputFile
	return cmd.Run()
}

func generateGeneralCalicoEnvs(config *CalicoConfig) []string {
	return []string{
		fmt.Sprintf("KUBE_NETWORK=%s", config.KubeNetwork),
		fmt.Sprintf("KUBECONFIG=%s", filepath.Join(config.ConfigPath, CalicoKubeConfigName)),
		fmt.Sprintf("NODENAME=%s", config.Hostname),
		fmt.Sprintf("CALICO_K8S_NODE_REF=%s", config.Hostname),

		fmt.Sprintf("IP=%s", config.NodeIP),
		fmt.Sprintf("USE_POD_CIDR=%t", autoConfigureIpam(config.IpamType)),
	}
}

// reserveCalicoVIP reserves a source VIP for kube-proxy
// If successful, the VIP is stored to config.CNICommonConfig.VIPAddress.
func reserveCalicoVIP(ctx context.Context, config *CalicoConfig) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		// calico-node is creating an endpoint named Calico_ep for this purpose
		endpoint, err := hcsshim.GetHNSEndpointByName("Calico_ep")
		if err != nil {
			logrus.WithError(err).Warning("Can't find Calico_ep HNS endpoint, retrying")
			return false, nil
		}
		config.CNICommonConfig.VIPAddress = endpoint.IPAddress.String()
		return true, nil
	})
}

// Get latest stored reboot
func (c *Calico) getStoredLastBootTime() (string, error) {
	lastRebootPath := filepath.Join(c.CNICfg.ConfigPath, "lastBootTime.txt")
	lastStoredBoot, err := os.ReadFile(lastRebootPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		} else {
			return "", err
		}
	}
	return string(lastStoredBoot), nil
}

// Set last boot time on the registry
func (c *Calico) setStoredLastBootTime(lastBootTime string) error {
	lastRebootPath := filepath.Join(c.CNICfg.ConfigPath, "lastBootTime.txt")
	err := os.WriteFile(lastRebootPath, []byte(lastBootTime), 0644)
	if err != nil {
		return err
	}
	return nil
}

// Check if the node was rebooted
func (c *Calico) isNodeRebooted() (bool, error) {
	tickCountSinceBoot := windows.DurationSinceBoot()
	bootTime := time.Now().Add(-tickCountSinceBoot)
	lastReboot := bootTime.Format(time.UnixDate)
	prevLastReboot, err := c.getStoredLastBootTime()
	if err != nil {
		return true, err
	}
	if lastReboot == prevLastReboot {
		return false, nil
	}
	err = c.setStoredLastBootTime(lastReboot)
	return true, err
}
