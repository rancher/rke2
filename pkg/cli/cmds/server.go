package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/rancher/spur/cli"
)

const rke2Path = "/var/lib/rancher/rke2"

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
			Hide:    true,
			Default: cmds.DisableItems,
		},
		"disable-selinux":            drop,
		"disable-scheduler":          drop,
		"disable-cloud-controller":   drop,
		"disable-network-policy":     drop,
		"disable-kube-proxy":         drop,
		"node-name":                  copy,
		"with-node-id":               drop,
		"node-label":                 copy,
		"node-taint":                 copy,
		"docker":                     copy,
		"container-runtime-endpoint": copy,
		"pause-image":                drop,
		"private-registry":           copy,
		"node-ip":                    copy,
		"node-external-ip":           drop,
		"resolv-conf":                copy,
		"flannel-iface":              drop,
		"flannel-conf":               drop,
		"kubelet-arg":                copy,
		"kube-proxy-arg":             copy,
		"rootless":                   drop,
		"agent-token":                copy,
		"agent-token-file":           copy,
		"server":                     copy,
		"secrets-encryption":         copy,
		"no-flannel":                 drop,
		"no-deploy":                  drop,
		"cluster-secret":             drop,
		"protect-kernel-defaults":    copy,
		"snapshotter":				  copy,
	})
)

func NewServerCommand() *cli.Command {
	cmd := k3sServerBase
	cmd.Flags = append(cmd.Flags, commonFlag...)
	return cmd
}

func ServerRun(ctx *cli.Context) error {
	return rke2.Server(ctx, config)
}
