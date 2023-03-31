//go:build windows
// +build windows

package windows

import (
	opv1 "github.com/tigera/operator/api/v1"
)

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
	OverlayNetName        string
	Mode                  string
	Hostname              string
	KubeNetwork           string
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
	KubeConfig            *CalicoKubeConfig
	Interface             string
}

type CalicoKubeConfig struct {
	CertificateAuthority string
	Server               string
	Token                string
	Path                 string
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
