package cmds

import (
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/etcdsnapshot"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/urfave/cli"
)

const defaultSnapshotRentention = 5

func NewEtcdSnapshotCommand() cli.Command {
	cmds.ServerConfig.ClusterInit = true
	k3sOpts := K3SFlagSet{
		"config":          copyFlag,
		"debug":           copyFlag,
		"log":             copyFlag,
		"alsologtostderr": copyFlag,
		"node-name":       copyFlag,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: rke2Path,
		},
		"etcd-server": {
			Default: "https://127.0.0.1:9345",
		},
		"etcd-token":         copyFlag,
		"name":               copyFlag,
		"dir":                copyFlag,
		"snapshot-compress":  copyFlag,
		"snapshot-retention": copyFlag,
		"s3":                 copyFlag,
		"s3-access-key":      copyFlag,
		"s3-bucket":          copyFlag,
		"s3-config-secret":   copyFlag,
		"s3-endpoint":        copyFlag,
		"s3-endpoint-ca":     copyFlag,
		"s3-folder":          copyFlag,
		"s3-insecure":        copyFlag,
		"s3-proxy":           copyFlag,
		"s3-region":          copyFlag,
		"s3-secret-key":      copyFlag,
		"s3-session-token":   copyFlag,
		"s3-skip-ssl-verify": copyFlag,
		"s3-timeout":         copyFlag,
	}
	subcommandOpts := map[string]K3SFlagSet{
		"ls": {
			"output": copyFlag,
		},
	}

	command := cmds.NewEtcdSnapshotCommands(
		etcdsnapshot.Delete,
		etcdsnapshot.List,
		etcdsnapshot.Prune,
		etcdsnapshot.Save)
	for i, subcommand := range command.Subcommands {
		if s, ok := subcommandOpts[subcommand.Name]; ok {
			k3sOpts.CopyInto(s)
			command.Subcommands[i] = mustCmdFromK3S(subcommand, s)
		} else {
			command.Subcommands[i] = mustCmdFromK3S(subcommand, k3sOpts)
		}
	}
	cmd := mustCmdFromK3S(command, k3sOpts)
	configfilearg.DefaultParser.ValidFlags[cmd.Name] = cmd.Flags
	return cmd
}
