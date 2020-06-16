package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/urfave/cli"
)

var (
	k3sAgentBase = mustCmdFromK3S(cmds.NewAgentCommand(AgentRun), map[string]*K3SFlagOption{
		"v":               hide,
		"vmodule":         hide,
		"log":             hide,
		"alsologtostderr": hide,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: rke2Path,
		},
		"token":                      nil,
		"token-file":                 nil,
		"disable-selinux":            drop,
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
		"server":                     nil,
		"no-flannel":                 drop,
		"cluster-secret":             drop,
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
