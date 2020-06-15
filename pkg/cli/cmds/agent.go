package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/urfave/cli"
)

var (
	k3sAgentBase = mustCmdFromK3S(cmds.NewAgentCommand(AgentRun), map[string]*K3SFlagOption{
		"v":               Hide,
		"vmodule":         Hide,
		"log":             Hide,
		"alsologtostderr": Hide,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: rke2Path,
		},
		"token":                      nil,
		"token-file":                 nil,
		"disable-selinux":            Drop,
		"node-name":                  nil,
		"with-node-id":               Drop,
		"node-label":                 nil,
		"node-taint":                 nil,
		"docker":                     nil,
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
		"server":                     nil,
		"no-flannel":                 Drop,
		"cluster-secret":             Drop,
	})
)

func NewAgentCommand() cli.Command {
	cmd := k3sAgentBase
	cmd.Flags = append(cmd.Flags, commonFlag...)
	return cmd
}

func AgentRun(app *cli.Context) error {
	return rke2.Agent(app, config)
}
