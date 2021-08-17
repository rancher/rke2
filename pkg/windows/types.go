// +build windows

package windows

import (
	"context"

	"github.com/rancher/k3s/pkg/daemons/config"
	daemonconfig "github.com/rancher/k3s/pkg/daemons/config"
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
