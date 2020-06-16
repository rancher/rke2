package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/urfave/cli"
)

var (
	config rke2.Config

	k3sServerBase = mustCmdFromK3S(cmds.NewServerCommand(ServerRun), map[string]*K3SFlagOption{
		"v":                 hide,
		"vmodule":           hide,
		"log":               hide,
		"alsologtostderr":   hide,
		"bind-address":      nil,
		"https-listen-port": drop,
		"advertise-address": nil,
		"advertise-port":    drop,
		"tls-san":           nil,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: rke2Path,
		},
		"disable-agent":                     hide,
		"cluster-cidr":                      nil,
		"service-cidr":                      nil,
		"cluster-init":                      nil,
		"cluster-reset":                     nil,
		"cluster-dns":                       nil,
		"cluster-domain":                    nil,
		"flannel-backend":                   drop,
		"token":                             nil,
		"token-file":                        nil,
		"write-kubeconfig":                  nil,
		"write-kubeconfig-mode":             nil,
		"kube-apiserver-arg":                nil,
		"kube-scheduler-arg":                nil,
		"kube-controller-arg":               drop,
		"kube-controller-manager-arg":       nil,
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
		"node-name":                  nil,
		"with-node-id":               drop,
		"node-label":                 nil,
		"node-taint":                 nil,
		"docker":                     nil,
		"container-runtime-endpoint": nil,
		"pause-image":                drop,
		"private-registry":           nil,
		"node-ip":                    nil,
		"node-external-ip":           drop,
		"resolv-conf":                nil,
		"flannel-iface":              drop,
		"flannel-conf":               drop,
		"kubelet-arg":                nil,
		"kube-proxy-arg":             nil,
		"rootless":                   drop,
		"agent-token":                nil,
		"agent-token-file":           nil,
		"server":                     nil,
		"secrets-encryption": {
			Hide:    false,
			Default: rke2ServerPath,
		},
		"no-flannel":     drop,
		"no-deploy":      drop,
		"cluster-secret": drop,
	})
)

func NewServerCommand() cli.Command {
	cmd := k3sServerBase
	cmd.Flags = append(cmd.Flags, commonFlag...)
	return cmd
}

func ServerRun(app *cli.Context) error {
	return rke2.Server(app, config)
}
