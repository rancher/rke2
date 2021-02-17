package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	DisableItems = "rke2-canal, rke2-coredns, rke2-ingress-nginx, rke2-kube-proxy, rke2-metrics-server"
	rke2Path     = "/var/lib/rancher/rke2"
)

var (
	config rke2.Config

	k3sServerBase = mustCmdFromK3S(cmds.NewServerCommand(ServerRun), map[string]*K3SFlagOption{
		"config":            copy,
		"debug":             copy,
		"v":                 hide,
		"vmodule":           hide,
		"log":               hide,
		"alsologtostderr":   hide,
		"bind-address":      copy,
		"https-listen-port": drop,
		"advertise-address": copy,
		"advertise-port":    drop,
		"tls-san":           copy,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: rke2Path,
		},
		"disable-agent":                     hide,
		"cluster-cidr":                      copy,
		"service-cidr":                      copy,
		"cluster-init":                      drop,
		"cluster-reset":                     copy,
		"cluster-reset-restore-path":        copy,
		"cluster-dns":                       copy,
		"cluster-domain":                    copy,
		"flannel-backend":                   drop,
		"token":                             copy,
		"token-file":                        copy,
		"write-kubeconfig":                  copy,
		"write-kubeconfig-mode":             copy,
		"kube-apiserver-arg":                copy,
		"kube-scheduler-arg":                copy,
		"kube-controller-arg":               drop,
		"kube-controller-manager-arg":       copy,
		"kube-cloud-controller-manager-arg": drop,
		"kube-cloud-controller-arg":         drop,
		"datastore-endpoint":                drop,
		"datastore-cafile":                  drop,
		"datastore-certfile":                drop,
		"datastore-keyfile":                 drop,
		"default-local-storage-path":        drop,
		"disable": {
			Usage: "(components) Do not deploy packaged components and delete any deployed components (valid items: " + DisableItems + ")",
		},
		"disable-selinux":             drop,
		"disable-scheduler":           copy,
		"disable-cloud-controller":    copy,
		"disable-network-policy":      drop,
		"disable-kube-proxy":          drop,
		"disable-api-server":          copy,
		"disable-controller-manager":  copy,
		"disable-etcd":                copy,
		"etcd-disable-snapshots":      copy,
		"etcd-snapshot-schedule-cron": copy,
		"etcd-snapshot-retention":     copy,
		"etcd-snapshot-dir":           copy,
		"etcd-snapshot-name":          copy,
		"node-name":                   copy,
		"with-node-id":                drop,
		"node-label":                  copy,
		"node-taint":                  copy,
		"docker":                      drop,
		"container-runtime-endpoint":  copy,
		"pause-image":                 drop,
		"private-registry":            copy,
		"node-ip":                     copy,
		"node-external-ip":            copy,
		"resolv-conf":                 copy,
		"flannel-iface":               drop,
		"flannel-conf":                drop,
		"kubelet-arg":                 copy,
		"kube-proxy-arg":              drop,
		"rootless":                    drop,
		"agent-token":                 copy,
		"agent-token-file":            copy,
		"server":                      copy,
		"secrets-encryption":          copy,
		"no-flannel":                  drop,
		"no-deploy":                   drop,
		"cluster-secret":              drop,
		"protect-kernel-defaults":     copy,
		"snapshotter":                 copy,
		"selinux":                     copy,
		"lb-server-port":              copy,
		"service-node-port-range":     copy,
		"etcd-expose-metrics":         copy,
		"airgap-extra-registry":       copy,
		"etcd-s3":                     drop,
		"etcd-s3-endpoint":            drop,
		"etcd-s3-endpoint-ca":         drop,
		"etcd-s3-skip-ssl-verify":     drop,
		"etcd-s3-access-key":          drop,
		"etcd-s3-secret-key":          drop,
		"etcd-s3-bucket":              drop,
		"etcd-s3-region":              drop,
		"etcd-s3-folder":              drop,
	})
)

func NewServerCommand() cli.Command {
	cmd := k3sServerBase
	cmd.Flags = append(cmd.Flags, commonFlag...)
	return cmd
}

func ServerRun(clx *cli.Context) error {
	switch profile {
	case "cis-1.5", "cis-1.6":
		if err := validateCISReqs("server"); err != nil {
			logrus.Fatal(err)
		}
		if err := setCISFlags(clx); err != nil {
			logrus.Fatal(err)
		}
	case "":
		logrus.Warn("not running in CIS mode")
	default:
		logrus.Fatal("invalid value provided for --profile flag")
	}

	return rke2.Server(clx, config)
}
