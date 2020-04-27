package cmds

import (
	"github.com/rancher/k3s/pkg/cli/agent"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/urfave/cli"
)

var (
	k3sAgentBase = mustCmdFromK3S(cmds.NewAgentCommand(agent.Run), map[string]*K3SFlagOption{
		"v":               Hide,
		"vmodule":         Hide,
		"log":             Hide,
		"alsologtostderr": Hide,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: "/var/lib/rancher/rke2",
		},
		"token":                      nil,
		"token-file":                 nil,
		"disable-selinux":            Drop,
		"node-name":                  nil,
		"with-node-id":               Drop,
		"node-label":                 nil,
		"node-taint":                 nil,
		"docker":                     Drop,
		"container-runtime-endpoint": Drop,
		"pause-image":                Drop,
		"private-registry":           Drop,
		"node-ip":                    nil,
		"node-external-ip":           Drop,
		"resolv-conf":                nil,
		"flannel-iface":              Drop,
		"flannel-conf":               Drop,
		"kubelet-arg":                nil,
		"kube-proxy-arg":             nil,
		"rootless":                   Drop,
		"server":                     nil,
		"no-flannel":                 Drop,
		"cluster-secret":             Drop,
	})
)

func NewAgentCommand() cli.Command {
	return k3sAgentBase
}
