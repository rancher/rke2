package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/rancher/rke2/pkg/windows"
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
	deprecatedFlags = []cli.Flag{
		&cli.StringFlag{
			Name:   "system-default-registry",
			Usage:  "(deprecated) This flag is no longer supported on agents",
			EnvVar: "RKE2_SYSTEM_DEFAULT_REGISTRY",
			Hidden: true,
		},
	}
)

func NewAgentCommand() cli.Command {
	cmd := k3sAgentBase
	cmd.Flags = append(cmd.Flags, commonFlag...)
	cmd.Flags = append(cmd.Flags, deprecatedFlags...)
	cmd.Subcommands = agentSubcommands()
	return cmd
}

func agentSubcommands() cli.Commands {
	subcommands := []cli.Command{
		// subcommands used by both windows/linux, none yet
	}

	// linux/windows only subcommands
	subcommands = append(subcommands, serviceSubcommand)

	return subcommands
}

func AgentRun(clx *cli.Context) error {
	validateCloudProviderName(clx, Agent)
	validateProfile(clx, Agent)
	if err := windows.StartService(); err != nil {
		return err
	}
	return rke2.Agent(clx, config)
}
