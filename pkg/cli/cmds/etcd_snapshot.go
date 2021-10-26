package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cli/etcdsnapshot"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/urfave/cli"
)

const defaultSnapshotRentention = 5

var k3sFlags = map[string]*K3SFlagOption{
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
	return k3sEtcdSnapshotBase
}

func EtcdSnapshotRun(clx *cli.Context) error {
	return rke2.EtcdSnapshot(clx, config)
}
