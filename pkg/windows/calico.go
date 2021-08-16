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
	"regexp"
	"strings"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/google/gopacket/routing"
	wapi "github.com/iamacarpet/go-win64api"
	"github.com/k3s-io/helm-controller/pkg/generated/controllers/helm.cattle.io"
	util2 "github.com/rancher/k3s/pkg/agent/util"
	"github.com/rancher/k3s/pkg/daemons/agent"
	"github.com/rancher/k3s/pkg/daemons/config"
	daemonconfig "github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Calico struct{}

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
func (c *Calico) Setup(ctx context.Context, dataDir string, nodeConfig *daemonconfig.Node, restConfig *rest.Config) (*CNIConfig, error) {
	calicoKubeConfig := CalicoKubeConfig{
		Server:               "https://127.0.0.1:6443",
		CertificateAuthority: filepath.Join("c:\\", dataDir, "agent", "server-ca.crt"),
		Token:                "",
		Path:                 filepath.Join("c:\\", dataDir, "agent", CalicoKubeConfigName),
	}

	client, err := coreClient(restConfig)
	if err != nil {
		return nil, err
	}

	secrets, err := client.CoreV1().Secrets(CalicoSystemNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, secret := range secrets.Items {
		if !isCalicoNodeToken(&secret) {
			continue
		}
		value, ok := secret.Data["token"]
		if !ok {
			continue
		}
		calicoKubeConfig.Token = string(value)
	}
	if calicoKubeConfig.Token == "" {
		return nil, errors.New("could not retrieve calico node token from the cluster")
	}

	cfg := &CNIConfig{NodeConfig: nodeConfig, NetworkName: "Calico", BindAddress: nodeConfig.AgentConfig.NodeIP}

	hc, err := helm.NewFactoryFromConfig(restConfig)
	if err != nil {
		return nil, err
	}

	if err := getDefaultConfig(cfg, dataDir, nodeConfig); err != nil {
		return nil, err
	}

	cfg.CalicoConfig.KubeConfig = calicoKubeConfig
	if err := getCNIConfigOverrides(cfg, hc); err != nil {
		return nil, err
	}

	agent.NetworkName = CalicoHnsNetworkName

	if err := createNodeNameFile(cfg, dataDir); err != nil {
		return nil, err
	}
	if err := createKubeConfig(cfg, dataDir); err != nil {
		return nil, err
	}

	if err := createCNIConfig(cfg); err != nil {
		return nil, err
	}

	logrus.Info("Generating HNS networks, please wait")
	if err := generateCalicoNetworks(cfg.CalicoConfig.NetworkingBackend); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Start starts the CNI services on the Windows node.
func (c *Calico) Start(ctx context.Context, config *CNIConfig) error {
	for {
		if err := startCalico(ctx, config.CalicoConfig); err != nil {
			continue
		}
		break
	}
	go startFelix(ctx, config.CalicoConfig)

	return nil
}

// createNodeNameFile creates the node name file required by the CNI.
func createNodeNameFile(config *CNIConfig, dataDir string) error {
	return util2.WriteFile(config.CalicoConfig.KubeConfig.Path, config.NodeConfig.AgentConfig.NodeName)
}

// createKubeConfig creates the kube config required by the CNI.
func createKubeConfig(config *CNIConfig, dataDir string) error {
	kubeTemplate, err := parseTemplateFromConfig(calicoKubeConfigTemplate, config)
	if err != nil {
		return err
	}
	return util2.WriteFile(filepath.Join("c:\\", dataDir, "agent", CalicoKubeConfigName), kubeTemplate)
}

// createCNIConfig creates the CNI config for the CNI.
func createCNIConfig(config *CNIConfig) error {
	parsedTemplate, err := parseTemplateFromConfig(calicoConfigTemplate, config)
	if err != nil {
		return err
	}
	return util2.WriteFile(filepath.Join(config.NodeConfig.AgentConfig.CNIConfDir, CalicoConfigName), parsedTemplate)
}

// getDefaultConfig sets the default configuration.
func getDefaultConfig(config *CNIConfig, dataDir string, nodeConfig *config.Node) error {
	platformType, err := getPlatformType()
	if err != nil {
		return err
	}

	calicoCfg := &CalicoConfig{
		Name:                  "Calico",
		Hostname:              nodeConfig.AgentConfig.NodeName,
		NodeNameFile:          filepath.Join("c:\\", dataDir, "agent", CalicoNodeNameFileName),
		KubeNetwork:           "Calico.*",
		Mode:                  "vxlan",
		ServiceCIDR:           nodeConfig.AgentConfig.ServiceCIDR.String(),
		DNSServers:            nodeConfig.AgentConfig.ClusterDNS.String(),
		DNSSearch:             "svc." + nodeConfig.AgentConfig.ClusterDomain,
		DatastoreType:         "kubernetes",
		NetworkingBackend:     "vxlan",
		Platform:              platformType,
		StartUpValidIPTimeout: 90,
		LogDir:                "",
		IP:                    "autodetect",
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
	config.CalicoConfig = calicoCfg
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

func deleteAllNetworksOnNodeRestart(backend string) error {
	logrus.Debug("Deleting networks.")
	backends := map[string]bool{
		"windows-bgp": true,
		"vxlan":       true,
	}

	if backends[backend] {
		networks, err := hcsshim.HNSListNetworkRequest("GET", "", "")
		if err != nil {
			return err
		}

		for _, n := range networks {
			if n.Name != "nat" {
				_, err = n.Delete()
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func checkForCorrectInterface() (bool, error) {
	logrus.Debug("Getting all interfaces")
	iFaces, err := net.Interfaces()
	if err != nil {
		return false, err
	}

	for _, iFace := range iFaces {
		addrs, err := iFace.Addrs()
		if err != nil {
			return false, err
		}
		for _, addr := range addrs {
			if addr.(*net.IPNet).IP.To4() != nil {
				match1, _ := regexp.Match("(^127\\.0\\.0\\.)", addr.(*net.IPNet).IP)
				match2, _ := regexp.Match("(^169\\.254\\.)", addr.(*net.IPNet).IP)
				if !(match1 || match2) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func generateCalicoNetworks(backend string) error {
	if err := deleteAllNetworksOnNodeRestart(backend); err != nil {
		return err
	}

	good, err := checkForCorrectInterface()
	if err != nil {
		return err
	}
	if !good {
		return fmt.Errorf("no interfaces found")
	}

	if err := createExternalNetwork(backend); err != nil {
		return err
	}

	logrus.Debug("Waiting for management ip..")
	mgmt := waitForManagementIP(CalicoHnsNetworkName)
	platform, err := getPlatformType()
	if err != nil {
		return err
	}
	if platform == "ec2" || platform == "gce" {
		err := setMetaDataServerRoute(mgmt)
		if err != nil {
			return err
		}
	}
	if backend == "windows-bgp" {
		// Not supported yet, this does work.
		_ = wapi.StopService("RemoteAccess")
		_ = wapi.StartService("RemoteAccess")
	}
	return nil
}

func checkIfNetworkExists(n string) bool {
	if _, err := hcsshim.GetHNSNetworkByName(n); err != nil {
		return false
	}
	return true
}

func createExternalNetwork(backend string) error {
	logrus.Debug("Creating external network")
	for !(checkIfNetworkExists(CalicoHnsNetworkName)) {
		logrus.Debugf("Networking doesn't exist yet: %s ", CalicoHnsNetworkName)
		var network hcsshim.HNSNetwork
		if backend == "vxlan" {
			logrus.Debugf("Backend is vxlan")
			// Ignoring the return because both true and false without an error represent that the firewall rule
			// was created or already exists
			if _, err := wapi.FirewallRuleAdd(
				"OverlayTraffic4789UDP",
				"Overlay network traffic UDP",
				"",
				"4789",
				wapi.NET_FW_IP_PROTOCOL_UDP,
				wapi.NET_FW_PROFILE2_ALL,
			); err != nil {
				logrus.Debugf("error creating firewall rules: %s", err)
				return err
			}
			logrus.Debug("Creating VXLAN network")
			network = hcsshim.HNSNetwork{
				Type: "Overlay",
				Name: CalicoHnsNetworkName,
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
			network = hcsshim.HNSNetwork{
				Type: "L2Bridge",
				Name: CalicoHnsNetworkName,
				Subnets: []hcsshim.Subnet{
					{
						AddressPrefix:  "192.168.255.0/30",
						GatewayAddress: "192.168.255.1",
					},
				},
			}
		}

		logrus.Debugf("Creating network.")
		if _, err := network.Create(); err != nil {
			logrus.Debugf("waiting for network %s", err)
			time.Sleep(1 * time.Second)
		}
	}
	return nil
}

func getPlatformType() (string, error) {
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

func waitForManagementIP(networkName string) string {
	for {
		network, err := hcsshim.GetHNSNetworkByName(networkName)
		if err != nil {
			logrus.Debugf("error getting management ip: %s", err)
			continue
		}
		return network.ManagementIP
	}
}

func setMetaDataServerRoute(mgmt string) error {
	ip := net.ParseIP(mgmt)
	if ip == nil {
		return fmt.Errorf("not a valid ip")
	}

	metaIp := net.ParseIP("169.254.169.254/32")

	router, err := routing.New()
	if err != nil {
		return err
	}

	route, _, preferredSrc, err := router.Route(ip)
	if err != nil {
		return err
	}
	_, _, _, err = router.RouteWithSrc(route.HardwareAddr, preferredSrc, metaIp)
	return err
}

func generateGeneralCalicoEnvs(config *CalicoConfig) []string {
	return []string{
		fmt.Sprintf("KUBE_NETWORK=%s", config.KubeNetwork),
		fmt.Sprintf("KUBECONFIG=%s", config.KubeConfig.Path),
		fmt.Sprintf("K8S_SERVICE_CIDR=%s", config.ServiceCIDR),
		fmt.Sprintf("NODENAME=%s", config.Hostname),

		fmt.Sprintf("CALICO_NETWORKING_BACKEND=%s", config.NetworkingBackend),
		fmt.Sprintf("CALICO_DATASTORE_TYPE=%s", config.DatastoreType),
		fmt.Sprintf("CALICO_K8S_NODE_REF=%s", config.Hostname),
		fmt.Sprintf("CALICO_LOG_DIR=%s", config.LogDir),

		fmt.Sprintf("DNS_NAME_SERVERS=%s", config.DNSServers),
		fmt.Sprintf("DNS_SEARCH=%s", config.DNSSearch),

		fmt.Sprintf("ETCD_ENDPOINTS=%s", config.Felix.Vxlanvni),
		fmt.Sprintf("ETCD_KEY_FILE=%s", config.Felix.Metadataaddr),
		fmt.Sprintf("ETCD_CERT_FILE=%s", config.Felix.Vxlanvni),
		fmt.Sprintf("ETCD_CA_CERT_FILE=%s", config.Felix.Metadataaddr),

		fmt.Sprintf("CNI_BIN_DIR=%s", config.CNI.BinDir),
		fmt.Sprintf("CNI_CONF_DIR=%s", config.CNI.ConfDir),
		fmt.Sprintf("CNI_CONF_FILENAME=%s", config.CNI.ConfFileName),
		fmt.Sprintf("CNI_IPAM_TYPE=%s", config.CNI.IpamType),

		fmt.Sprintf("FELIX_LOGSEVERITYFILE=%s", config.Felix.LogSeverityFile),
		fmt.Sprintf("FELIX_LOGSEVERITYSYS=%s", config.Felix.LogSeveritySys),

		fmt.Sprintf("STARTUP_VALID_IP_TIMEOUT=90"),
		fmt.Sprintf("IP=%s", config.IP),
		fmt.Sprintf("USE_POD_CIDR=%t", autoConfigureIpam(config.CNI.IpamType)),

		fmt.Sprintf("VXLAN_VNI=%s", config.Felix.Vxlanvni),
	}
}

// getCNIConfigOverrides overrides the default values set for the CNI.
func getCNIConfigOverrides(cniConfig *CNIConfig, hc *helm.Factory) error {
	if _, err := hc.Helm().V1().HelmChartConfig().Get(metav1.NamespaceSystem, CalicoChart, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to check for %s HelmChartConfig", CalicoChart)
	}
	logrus.Debug("custom calico configuration isn't currently supported")
	return nil
}

func coreClient(restConfig *rest.Config) (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(restConfig)
}

func isCalicoNodeToken(s *v1.Secret) bool {
	if v, ok := s.Annotations["kubernetes.io/service-account.name"]; ok && v == calicoNode {
		return true
	}
	return false
}
