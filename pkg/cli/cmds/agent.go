package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/urfave/cli"
)

var (
	k3sAgentBase = mustCmdFromK3S(cmds.NewAgentCommand(AgentRun), map[string]*K3SFlagOption{
		"v":                          hide,
		"vmodule":                    hide,
		"log":                        hide,
		"alsologtostderr":            hide,
		"data-dir":                   copy,
		"token":                      copy,
		"token-file":                 copy,
		"disable-selinux":            drop,
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
		"server":                     copy,
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
