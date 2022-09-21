//go:build windows
// +build windows

package windows

import (
	"context"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"k8s.io/client-go/rest"
)

type CNI interface {
	Setup(context.Context, string, *daemonconfig.Node, *rest.Config) (*CNIConfig, error)
	Start(context.Context, *CNIConfig) error
}

type CNIConfig struct {
	NodeConfig   *config.Node
	CalicoConfig *CalicoConfig
	NetworkName  string
	BindAddress  string
}

type FelixConfig struct {
	Metadataaddr    string
	Vxlanvni        string
	MacPrefix       string
	LogSeverityFile string
	LogSeveritySys  string
}

type CalicoCNIConfig struct {
	BinDir       string
	ConfDir      string
	IpamType     string
	ConfFileName string
	Version      string
}

type CalicoConfig struct {
	Name                  string
	Mode                  string
	Hostname              string
	KubeNetwork           string
	NetworkingBackend     string
	ServiceCIDR           string
	DNSServers            string
	DNSSearch             string
	DatastoreType         string
	NodeNameFile          string
	Platform              string
	StartUpValidIPTimeout int
	IP                    string
	IPAutoDetectionMethod string
	LogDir                string
	Felix                 FelixConfig
	CNI                   CalicoCNIConfig
	ETCDEndpoints         string
	ETCDKeyFile           string
	ETCDCertFile          string
	ETCDCaCertFile        string
	KubeConfig            CalicoKubeConfig
}

type CalicoKubeConfig struct {
	CertificateAuthority string
	Server               string
	Token                string
	Path                 string
}

// Subset of Calico configuaration used to extract custom configuration
// Based off of https://github.com/tigera/operator/blob/master/api/v1/installation_types.go, but converted to yaml
type CalicoInstallation struct {
	Installation CalicoInstallationSpec `yaml:"installation,omitempty"`
}

type CalicoInstallationSpec struct {
	CalicoNetwork            CalicoNetwork     `yaml:"calicoNetwork,omitempty"`
	FlexVolumePath           string            `yaml:"flexVolumePath,omitempty"`
	ControlPlaneNodeSelector map[string]string `yaml:"controlPlaneNodeSelector,omitempty"`
}

type CalicoNetwork struct {
	NodeAddressAutodetectionV4 *NodeAddressAutodetection `yaml:"nodeAddressAutodetectionV4,omitempty"`
}

// NodeAddressAutodetectionV4
type NodeAddressAutodetection struct {
	// FirstFound uses default interface matching parameters to select an interface, performing best-effort
	// filtering based on well-known interface names.
	// +optional
	FirstFound bool `yaml:"firstFound,omitempty"`

	// Interface enables IP auto-detection based on interfaces that match the given regex.
	// +optional
	Interface string `yaml:"interface,omitempty"`

	// SkipInterface enables IP auto-detection based on interfaces that do not match
	// the given regex.
	// +optional
	SkipInterface string `yaml:"skipInterface,omitempty"`

	// CanReach enables IP auto-detection based on which source address on the node is used to reach the
	// specified IP or domain.
	// +optional
	CanReach string `yaml:"canReach,omitempty"`

	// CIDRS enables IP auto-detection based on which addresses on the nodes are within
	// one of the provided CIDRs.
	CIDRS []string `yaml:"cidrs,omitempty"`
}
