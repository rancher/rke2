package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/urfave/cli"
)

var (
	k3sEtcdSnapshotBase = mustCmdFromK3S(cmds.NewEtcdSnapshotCommand(EtcdSnapshotRun, []cli.Command{}), map[string]*K3SFlagOption{
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
	})
)

func NewEtcdSnapshotCommand() cli.Command {
	cmd := k3sEtcdSnapshotBase
	cmd.Flags = append(cmd.Flags, commonFlag...)
	return cmd
}

func EtcdSnapshotRun(clx *cli.Context) error {
	return rke2.EtcdSnapshot(clx, config)
}
