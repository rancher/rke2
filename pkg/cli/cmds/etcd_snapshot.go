package cmds

import (
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/etcdsnapshot"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/urfave/cli"
)

const defaultSnapshotRentention = 5

func NewEtcdSnapshotCommand() cli.Command {
	cmds.ServerConfig.DatastoreEndpoint = "etcd"
	k3sOpts := K3SFlagSet{
		"config":          copy,
		"debug":           copy,
		"log":             copy,
		"alsologtostderr": copy,
		"node-name":       copy,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: rke2Path,
		},
		"name":               copy,
		"dir":                copy,
		"snapshot-compress":  copy,
		"s3":                 copy,
		"s3-endpoint":        copy,
		"s3-endpoint-ca":     copy,
		"s3-skip-ssl-verify": copy,
		"s3-access-key":      copy,
		"s3-secret-key":      copy,
		"s3-bucket":          copy,
		"s3-region":          copy,
		"s3-folder":          copy,
		"s3-insecure":        copy,
		"s3-timeout":         copy,
	}
	subcommandOpts := map[string]K3SFlagSet{
		"ls": {
			"output": copy,
		},
		"prune": {
			"snapshot-retention": copy,
		},
	}

	command := cmds.NewEtcdSnapshotCommands(
		etcdsnapshot.Run,
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
