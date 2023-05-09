package cmds

import (
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/etcdsnapshot"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/urfave/cli"
)

const defaultSnapshotRentention = 5

var k3sFlags = K3SFlagSet{
	"config":          copyFlag,
	"debug":           copyFlag,
	"log":             copyFlag,
	"alsologtostderr": copyFlag,
	"node-name":       copyFlag,
	"data-dir": {
		Usage:   "(data) Folder to hold state",
		Default: rke2Path,
	},
	"name":               copyFlag,
	"dir":                copyFlag,
	"snapshot-compress":  copyFlag,
	"s3":                 copyFlag,
	"s3-endpoint":        copyFlag,
	"s3-endpoint-ca":     copyFlag,
	"s3-skip-ssl-verify": copyFlag,
	"s3-access-key":      copyFlag,
	"s3-secret-key":      copyFlag,
	"s3-bucket":          copyFlag,
	"s3-region":          copyFlag,
	"s3-folder":          copyFlag,
	"s3-insecure":        copyFlag,
	"s3-timeout":         copyFlag,
}

var subcommands = []cli.Command{
	{
		Name:            "delete",
		Usage:           "Delete given snapshot(s)",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          etcdsnapshot.Delete,
		Flags:           cmds.EtcdSnapshotFlags,
	},
	{
		Name:            "ls",
		Aliases:         []string{"list", "l"},
		Usage:           "List snapshots",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          etcdsnapshot.List,
		Flags:           cmds.EtcdSnapshotFlags,
	},
	{
		Name:            "prune",
		Usage:           "Remove snapshots that exceed the configured retention count",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          etcdsnapshot.Prune,
		Flags: append(cmds.EtcdSnapshotFlags, &cli.IntFlag{
			Name:        "snapshot-retention",
			Usage:       "(db) Number of snapshots to retain. Default: 5",
			Destination: &cmds.ServerConfig.EtcdSnapshotRetention,
			Value:       defaultSnapshotRentention,
		}),
	},
	{
		Name:            "save",
		Usage:           "Trigger an immediate etcd snapshot",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          etcdsnapshot.Run,
		Flags:           cmds.EtcdSnapshotFlags,
	},
}

var (
	k3sEtcdSnapshotBase = mustCmdFromK3S(cmds.NewEtcdSnapshotCommand(EtcdSnapshotRun, subcommands), k3sFlags)
)

func NewEtcdSnapshotCommand() cli.Command {
	cmd := k3sEtcdSnapshotBase
	configfilearg.DefaultParser.ValidFlags[cmd.Name] = cmd.Flags
	return cmd
}

func EtcdSnapshotRun(clx *cli.Context) error {
	return rke2.EtcdSnapshot(clx, config)
}
