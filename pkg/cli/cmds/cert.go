package cmds

import (
	"io/ioutil"

	"github.com/k3s-io/k3s/pkg/cli/cert"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/urfave/cli"
)

var certSubcommands = []cli.Command{
	{
		Name:            "rotate",
		Usage:           "Rotate " + version.Program + " component certificates on disk",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          Rotate,
		Flags:           cmds.CertRotateCommandFlags,
	},
	{
		Name:            "rotate-ca",
		Usage:           "Write updated " + version.Program + " CA certificates to the datastore",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          cert.RotateCA,
		Flags:           cmds.CertRotateCACommandFlags,
	},
}

func NewCertCommand() cli.Command {
	k3sOpts := K3SFlagSet{}
	subCommandOpts := map[string]K3SFlagSet{
		"rotate": {
			"alsologtostderr": copy,
			"config":          copy,
			"debug":           copy,
			"log":             copy,
			"service":         copy,
			"data-dir": {
				Usage:   "(data) Folder to hold state",
				Default: rke2Path,
			},
		},
		"rotate-ca": {
			"server": copy,
			"path":   copy,
			"force":  copy,
		},
	}

	command := cmds.NewCertCommand(certSubcommands)
	command.Usage = "Manage RKE2 certificates"
	configfilearg.DefaultParser.ValidFlags[command.Name] = command.Flags
	for i, subcommand := range command.Subcommands {
		if s, ok := subCommandOpts[subcommand.Name]; ok {
			k3sOpts.CopyInto(s)
			command.Subcommands[i] = mustCmdFromK3S(subcommand, s)
		} else {
			command.Subcommands[i] = mustCmdFromK3S(subcommand, k3sOpts)
		}
	}
	return mustCmdFromK3S(command, nil)
}

func Rotate(clx *cli.Context) error {
	dataDir := clx.String("data-dir")
	if dataDir == "" {
		dataDir = rke2Path
	}
	if err := ioutil.WriteFile(rke2.ForceRestartFile(dataDir), []byte{}, 0600); err != nil {
		return err
	}
	return cert.Rotate(clx)
}
