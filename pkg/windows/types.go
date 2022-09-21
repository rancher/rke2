//go:build windows
// +build windows

package windows

import (
	"context"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	opv1 "github.com/tigera/operator/api/v1"
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

// Stub of Calico configuaration used to extract custom configuration
// Based off of https://github.com/tigera/operator/blob/master/api/v1/installation_types.go
type CalicoInstallation struct {
	Installation CalicoInstallationSpec `json:"installation,omitempty"`
}

type CalicoInstallationSpec struct {
	CalicoNetwork            opv1.CalicoNetworkSpec `json:"calicoNetwork,omitempty"`
	FlexVolumePath           string                 `json:"flexVolumePath,omitempty"`
	ControlPlaneNodeSelector map[string]string      `json:"controlPlaneNodeSelector,omitempty"`
}
