package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/urfave/cli"
)

var (
	config rke2.Config

	k3sServerBase = mustCmdFromK3S(cmds.NewServerCommand(ServerRun), map[string]*K3SFlagOption{
		"v":                 Hide,
		"vmodule":           Hide,
		"log":               Hide,
		"alsologtostderr":   Hide,
		"bind-address":      nil,
		"https-listen-port": Drop,
		"advertise-address": nil,
		"advertise-port":    Drop,
		"tls-san":           nil,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: "/var/lib/rancher/rke2",
		},
		"disable-agent":                     Hide,
		"cluster-cidr":                      nil,
		"service-cidr":                      nil,
		"cluster-init":                      nil,
		"cluster-reset":                     nil,
		"cluster-dns":                       nil,
		"cluster-domain":                    nil,
		"flannel-backend":                   Drop,
		"token":                             nil,
		"token-file":                        nil,
		"write-kubeconfig":                  nil,
		"write-kubeconfig-mode":             nil,
		"kube-apiserver-arg":                nil,
		"kube-scheduler-arg":                nil,
		"kube-controller-arg":               Drop,
		"kube-controller-manager-arg":       nil,
		"kube-cloud-controller-manager-arg": Drop,
		"kube-cloud-controller-arg":         Drop,
		"datastore-endpoint":                Drop,
		"datastore-cafile":                  Drop,
		"datastore-certfile":                Drop,
		"datastore-keyfile":                 Drop,
		"default-local-storage-path":        Drop,
		"disable": {
			Hide:    true,
			Default: cmds.DisableItems,
		},
		"disable-selinux":            Drop,
		"disable-scheduler":          Drop,
		"disable-cloud-controller":   Drop,
		"disable-network-policy":     Drop,
		"disable-kube-proxy":         Drop,
		"node-name":                  nil,
		"with-node-id":               Drop,
		"node-label":                 nil,
		"node-taint":                 nil,
		"docker":                     Drop,
		"container-runtime-endpoint": nil,
		"pause-image":                Drop,
		"private-registry":           nil,
		"node-ip":                    nil,
		"node-external-ip":           Drop,
		"resolv-conf":                nil,
		"flannel-iface":              Drop,
		"flannel-conf":               Drop,
		"kubelet-arg":                nil,
		"kube-proxy-arg":             nil,
		"rootless":                   Drop,
		"agent-token":                nil,
		"agent-token-file":           nil,
		"server":                     nil,
		"secrets-encryption":         nil,
		"no-flannel":                 Drop,
		"no-deploy":                  Drop,
		"cluster-secret":             Drop,
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
