package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	k3sAgentBase = mustCmdFromK3S(cmds.NewAgentCommand(AgentRun), map[string]*K3SFlagOption{
		"config":          copy,
		"debug":           copy,
		"v":               hide,
		"vmodule":         hide,
		"log":             hide,
		"alsologtostderr": hide,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: rke2Path,
		},
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
		"protect-kernel-defaults":    copy,
		"snapshotter":                copy,
		"selinux":                    copy,
	})
)

func NewAgentCommand() cli.Command {
	cmd := k3sAgentBase
	cmd.Flags = append(cmd.Flags, commonFlag...)
	return cmd
}

func AgentRun(clx *cli.Context) error {
	if clx.String("profile") == "" {
		logrus.Warn("not running in CIS 1.5 mode")
	}
	return rke2.Agent(clx, config)
}
