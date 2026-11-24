//go:build windows
// +build windows

package windows

import (
	"context"

	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"k8s.io/client-go/rest"
)

type CNIPlugin interface {
	Setup(ctx context.Context, nodeConfig *daemonconfig.Node, restConfig *rest.Config, dataDir string) error
	Start(ctx context.Context) error
	GetConfig() *CNICommonConfig
}

type KubeConfig struct {
	CertificateAuthority string
	Server               string
	Token                string
	Path                 string
}

type CNICommonConfig struct {
	Name           string
	OverlayNetName string
	OverlayEncap   string
	Hostname       string
	ConfigPath     string
	CNIConfDir     string
	CNIBinDir      string
	ClusterCIDR    string
	ServiceCIDR    string
	NodeIP         string
	VxlanVNI       string
	VxlanPort      string
	Interface      string
	IpamType       string
	CNIVersion     string
	VIPAddress     string
	KubeConfig     *KubeConfig
}

type CalicoConfig struct {
	CNICommonConfig       // embedded struct
	KubeNetwork           string
	DNSServers            string
	DNSSearch             string
	DatastoreType         string
	NodeNameFile          string
	Platform              string
	IPAutoDetectionMethod string
	ETCDEndpoints         string
	ETCDKeyFile           string
	ETCDCertFile          string
	ETCDCaCertFile        string
}

type FlannelConfig struct {
	CNICommonConfig // embedded struct
}

// Stub of Calico configuration used to extract user-provided overrides
// Based off of https://github.com/tigera/operator/blob/master/api/v1/installation_types.go
type CalicoInstallation struct {
	Installation CalicoInstallationSpec `json:"installation,omitempty"`
}

type CalicoInstallationSpec struct {
	CalicoNetwork            CalicoNetworkSpec `json:"calicoNetwork,omitempty"`
	FlexVolumePath           string            `json:"flexVolumePath,omitempty"`
	ControlPlaneNodeSelector map[string]string `json:"controlPlaneNodeSelector,omitempty"`
}

type CalicoNetworkSpec struct {
	BGP                        *BGPOption                `json:"bgp,omitempty"`
	NodeAddressAutodetectionV4 *NodeAddressAutodetection `json:"nodeAddressAutodetectionV4,omitempty"`
	NodeAddressAutodetectionV6 *NodeAddressAutodetection `json:"nodeAddressAutodetectionV6,omitempty"`
}

type BGPOption string

const (
	BGPEnabled  BGPOption = "Enabled"
	BGPDisabled BGPOption = "Disabled"
)

type NodeAddressAutodetection struct {
	FirstFound    *bool                          `json:"firstFound,omitempty"`
	Kubernetes    *KubernetesAutodetectionMethod `json:"kubernetes,omitempty"`
	Interface     string                         `json:"interface,omitempty"`
	SkipInterface string                         `json:"skipInterface,omitempty"`
	CanReach      string                         `json:"canReach,omitempty"`
	CIDRS         []string                       `json:"cidrs,omitempty"`
}

type KubernetesAutodetectionMethod string

const (
	NodeInternalIP KubernetesAutodetectionMethod = "NodeInternalIP"
)
