package cmds

import (
	"strings"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/rancher/wrangler/pkg/slice"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	rke2Path = "/var/lib/rancher/rke2"
)

var (
	DisableItems = []string{"rke2-coredns", "rke2-ingress-nginx", "rke2-metrics-server"}
	CNIItems     = []string{"calico", "canal", "cilium"}

	config = rke2.Config{}

	serverFlag = []cli.Flag{
		&cli.StringSliceFlag{
			Name:   "cni",
			Usage:  "(networking) CNI Plugins to deploy, one of none, " + strings.Join(CNIItems, ", ") + "; optionally with multus as the first value to enable the multus meta-plugin (default: canal)",
			EnvVar: "RKE2_CNI",
		},
		&cli.BoolFlag{
			Name:   "enable-servicelb",
			Usage:  "(components) Enable rke2 default cloud controller manager's service controller",
			EnvVar: "RKE2_ENABLE_SERVICELB",
		},
	}

	k3sServerBase = mustCmdFromK3S(cmds.NewServerCommand(ServerRun), K3SFlagSet{
		"config":            copyFlag,
		"debug":             copyFlag,
		"v":                 hideFlag,
		"vmodule":           hideFlag,
		"log":               hideFlag,
		"alsologtostderr":   hideFlag,
		"bind-address":      copyFlag,
		"https-listen-port": dropFlag,
		"advertise-address": copyFlag,
		"advertise-port":    dropFlag,
		"tls-san":           copyFlag,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: rke2Path,
		},
		"disable-agent":                     hideFlag,
		"cluster-cidr":                      copyFlag,
		"service-cidr":                      copyFlag,
		"cluster-init":                      dropFlag,
		"cluster-reset":                     copyFlag,
		"cluster-reset-restore-path":        copyFlag,
		"cluster-dns":                       copyFlag,
		"cluster-domain":                    copyFlag,
		"flannel-backend":                   dropFlag,
		"token":                             copyFlag,
		"token-file":                        copyFlag,
		"write-kubeconfig":                  copyFlag,
		"write-kubeconfig-mode":             copyFlag,
		"kube-apiserver-arg":                copyFlag,
		"etcd-arg":                          copyFlag,
		"kube-scheduler-arg":                copyFlag,
		"kube-controller-arg":               dropFlag,
		"kube-controller-manager-arg":       copyFlag,
		"kube-cloud-controller-manager-arg": dropFlag,
		"kube-cloud-controller-arg":         dropFlag,
		"datastore-endpoint":                dropFlag,
		"datastore-cafile":                  dropFlag,
		"datastore-certfile":                dropFlag,
		"datastore-keyfile":                 dropFlag,
		"default-local-storage-path":        dropFlag,
		"disable": {
			Usage: "(components) Do not deploy packaged components and delete any deployed components (valid items: " + strings.Join(DisableItems, ", ") + ")",
		},
		"disable-scheduler":                 copyFlag,
		"disable-cloud-controller":          copyFlag,
		"disable-network-policy":            dropFlag,
		"disable-kube-proxy":                copyFlag,
		"disable-apiserver":                 copyFlag,
		"disable-controller-manager":        copyFlag,
		"disable-etcd":                      copyFlag,
		"etcd-disable-snapshots":            copyFlag,
		"etcd-snapshot-schedule-cron":       copyFlag,
		"etcd-snapshot-retention":           copyFlag,
		"etcd-snapshot-dir":                 copyFlag,
		"etcd-snapshot-name":                copyFlag,
		"etcd-snapshot-compress":            copyFlag,
		"node-name":                         copyFlag,
		"with-node-id":                      dropFlag,
		"node-label":                        copyFlag,
		"node-taint":                        copyFlag,
		"image-credential-provider-bin-dir": copyFlag,
		"image-credential-provider-config":  copyFlag,
		"docker":                            dropFlag,
		"container-runtime-endpoint":        copyFlag,
		"pause-image":                       dropFlag,
		"private-registry":                  copyFlag,
		"system-default-registry":           copyFlag,
		"node-ip":                           copyFlag,
		"node-external-ip":                  copyFlag,
		"resolv-conf":                       copyFlag,
		"flannel-iface":                     dropFlag,
		"flannel-conf":                      dropFlag,
		"flannel-cni-conf":                  dropFlag,
		"flannel-ipv6-masq":                 dropFlag,
		"flannel-external-ip":               dropFlag,
		"multi-cluster-cidr":                hideFlag,
		"egress-selector-mode":              copyFlag,
		"kubelet-arg":                       copyFlag,
		"kube-proxy-arg":                    copyFlag,
		"rootless":                          dropFlag,
		"prefer-bundled-bin":                dropFlag,
		"agent-token":                       copyFlag,
		"agent-token-file":                  copyFlag,
		"server":                            copyFlag,
		"secrets-encryption":                hideFlag,
		"protect-kernel-defaults":           copyFlag,
		"snapshotter":                       copyFlag,
		"selinux":                           copyFlag,
		"lb-server-port":                    copyFlag,
		"service-node-port-range":           copyFlag,
		"etcd-expose-metrics":               copyFlag,
		"airgap-extra-registry":             copyFlag,
		"etcd-s3":                           copyFlag,
		"etcd-s3-endpoint":                  copyFlag,
		"etcd-s3-endpoint-ca":               copyFlag,
		"etcd-s3-skip-ssl-verify":           copyFlag,
		"etcd-s3-access-key":                copyFlag,
		"etcd-s3-secret-key":                copyFlag,
		"etcd-s3-bucket":                    copyFlag,
		"etcd-s3-region":                    copyFlag,
		"etcd-s3-folder":                    copyFlag,
		"etcd-s3-insecure":                  copyFlag,
		"etcd-s3-timeout":                   copyFlag,
		"disable-helm-controller":           dropFlag,
		"enable-pprof":                      copyFlag,
		"servicelb-namespace":               copyFlag,
	})
)

func NewServerCommand() cli.Command {
	cmd := k3sServerBase
	cmd.Flags = append(cmd.Flags, serverFlag...)
	cmd.Flags = append(cmd.Flags, commonFlag...)
	configfilearg.DefaultParser.ValidFlags[cmd.Name] = cmd.Flags
	return cmd
}

func ServerRun(clx *cli.Context) error {
	validateCloudProviderName(clx, Server)
	validateProfile(clx, Server)
	validateCNI(clx)
	return rke2.Server(clx, config)
}

func validateCNI(clx *cli.Context) {
	cnis := []string{}
	for _, cni := range clx.StringSlice("cni") {
		for _, v := range strings.Split(cni, ",") {
			cnis = append(cnis, v)
		}
	}

	switch len(cnis) {
	case 0:
		cnis = append(cnis, "canal")
		fallthrough
	case 1:
		if cnis[0] == "multus" {
			logrus.Fatal("invalid value provided for --cni flag: multus must be used alongside another primary cni selection")
		}
		clx.Set("disable", "rke2-multus")
	case 2:
		if cnis[0] == "multus" {
			cnis = cnis[1:]
		} else {
			logrus.Fatal("invalid values provided for --cni flag: may only provide multiple values if multus is the first value")
		}
	default:
		logrus.Fatal("invalid values provided for --cni flag: may not provide more than two values")
	}

	switch {
	case cnis[0] == "none":
		fallthrough
	case slice.ContainsString(CNIItems, cnis[0]):
		for _, d := range CNIItems {
			if cnis[0] != d {
				clx.Set("disable", "rke2-"+d)
				clx.Set("disable", "rke2-"+d+"-crd")
			}
		}
	default:
		logrus.Fatal("invalid value provided for --cni flag")
	}
}
