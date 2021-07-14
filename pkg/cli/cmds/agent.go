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
		"token":                             copy,
		"token-file":                        copy,
		"disable-selinux":                   drop,
		"node-name":                         copy,
		"with-node-id":                      drop,
		"node-label":                        copy,
		"node-taint":                        copy,
		"image-credential-provider-bin-dir": copy,
		"image-credential-provider-config":  copy,
		"docker":                            drop,
		"container-runtime-endpoint":        copy,
		"pause-image":                       drop,
		"private-registry":                  copy,
		"node-ip":                           copy,
		"node-external-ip":                  copy,
		"resolv-conf":                       copy,
		"flannel-iface":                     drop,
		"flannel-conf":                      drop,
		"kubelet-arg":                       copy,
		"kube-proxy-arg":                    copy,
		"rootless":                          drop,
		"server":                            copy,
		"no-flannel":                        drop,
		"cluster-secret":                    drop,
		"protect-kernel-defaults":           copy,
		"snapshotter":                       copy,
		"selinux":                           copy,
		"lb-server-port":                    copy,
		"airgap-extra-registry":             copy,
	})
)

func NewAgentCommand() cli.Command {
	cmd := k3sAgentBase
	cmd.Flags = append(cmd.Flags, commonFlag...)
	return cmd
}

func AgentRun(clx *cli.Context) error {
	switch profile {
	case rke2.CISProfile:
		if err := validateCISReqs("agent"); err != nil {
			logrus.Fatal(err)
		}
		if err := setCISFlags(clx); err != nil {
			logrus.Fatal(err)
		}
	case "":
		logrus.Warn("not running in CIS 1.5 mode")
	default:
		logrus.Fatal("invalid value provided for --profile flag")
	}

	return rke2.Agent(clx, config)
}
