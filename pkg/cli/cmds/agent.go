package cmds

import (
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/rancher/rke2/pkg/windows"
	"github.com/urfave/cli"
)

var (
	k3sAgentBase = mustCmdFromK3S(cmds.NewAgentCommand(AgentRun), K3SFlagSet{
		"config":          copyFlag,
		"debug":           copyFlag,
		"v":               hideFlag,
		"vmodule":         hideFlag,
		"log":             hideFlag,
		"alsologtostderr": hideFlag,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: rke2Path,
		},
		"token":                             copyFlag,
		"token-file":                        copyFlag,
		"node-name":                         copyFlag,
		"with-node-id":                      copyFlag,
		"node-label":                        copyFlag,
		"node-taint":                        copyFlag,
		"image-credential-provider-bin-dir": copyFlag,
		"image-credential-provider-config":  copyFlag,
		"docker":                            dropFlag,
		"container-runtime-endpoint":        copyFlag,
		"pause-image":                       dropFlag,
		"private-registry":                  copyFlag,
		"node-ip":                           copyFlag,
		"node-external-ip":                  copyFlag,
		"resolv-conf":                       copyFlag,
		"flannel-iface":                     dropFlag,
		"flannel-conf":                      dropFlag,
		"flannel-cni-conf":                  dropFlag,
		"kubelet-arg":                       copyFlag,
		"kube-proxy-arg":                    copyFlag,
		"rootless":                          dropFlag,
		"prefer-bundled-bin":                dropFlag,
		"server":                            copyFlag,
		"protect-kernel-defaults":           copyFlag,
		"snapshotter":                       copyFlag,
		"selinux":                           copyFlag,
		"lb-server-port":                    copyFlag,
		"airgap-extra-registry":             copyFlag,
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
	configfilearg.DefaultParser.ValidFlags[cmd.Name] = cmd.Flags
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
