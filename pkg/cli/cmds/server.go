package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	DisableItems = "rke2-canal, rke2-coredns, rke2-ingress, rke2-kube-proxy, rke2-metrics-server"
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
		"cluster-init":                      copy,
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
		"disable-scheduler":           drop,
		"disable-cloud-controller":    drop,
		"disable-network-policy":      drop,
		"disable-kube-proxy":          drop,
		"etcd-disable-snapshots":      copy,
		"etcd-snapshot-schedule-cron": copy,
		"etcd-snapshot-retention":     copy,
		"etcd-snapshot-dir":           copy,
		"node-name":                   copy,
		"with-node-id":                drop,
		"node-label":                  copy,
		"node-taint":                  copy,
		"docker":                      copy,
		"container-runtime-endpoint":  copy,
		"pause-image":                 drop,
		"private-registry":            copy,
		"node-ip":                     copy,
		"node-external-ip":            drop,
		"resolv-conf":                 copy,
		"flannel-iface":               drop,
		"flannel-conf":                drop,
		"kubelet-arg":                 copy,
		"kube-proxy-arg":              copy,
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
	})
)

func NewServerCommand() cli.Command {
	cmd := k3sServerBase
	cmd.Flags = append(cmd.Flags, commonFlag...)
	return cmd
}

func ServerRun(clx *cli.Context) error {
	if profile == "" {
		logrus.Warn("not running in CIS 1.5 mode")
	}
	return rke2.Server(clx, config)
}
