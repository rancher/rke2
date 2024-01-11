//go:build windows
// +build windows

package windows

import (
	"context"

	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	opv1 "github.com/tigera/operator/api/v1"
	"k8s.io/client-go/rest"
)

type CNIPlugin interface {
	Setup(ctx context.Context, nodeConfig *daemonconfig.Node, restConfig *rest.Config, dataDir string) error
	Start(ctx context.Context) error
	GetConfig() *CNICommonConfig
	ReserveSourceVip(ctx context.Context) (string, error)
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
	CalicoNetwork            opv1.CalicoNetworkSpec `json:"calicoNetwork,omitempty"`
	FlexVolumePath           string                 `json:"flexVolumePath,omitempty"`
	ControlPlaneNodeSelector map[string]string      `json:"controlPlaneNodeSelector,omitempty"`
}
