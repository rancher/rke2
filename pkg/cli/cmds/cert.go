package cmds

import (
	"io/ioutil"

	"github.com/k3s-io/k3s/pkg/cli/cert"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/urfave/cli"
)

var k3sCertFlags = map[string]*K3SFlagOption{
	"config":          copy,
	"debug":           copy,
	"log":             copy,
	"alsologtostderr": copy,
	"service":         copy,
	"data-dir": {
		Usage:   "(data) Folder to hold state",
		Default: rke2Path,
	},
}

var certSubcommands = []cli.Command{
	{
		Name:            "rotate",
		Usage:           "Certificate Rotatation",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          CertificateRotationRun,
		Flags:           cmds.CertCommandFlags,
	},
}

var (
	k3sCertBase = mustCmdFromK3S(cmds.NewCertCommand(certSubcommands), k3sCertFlags)
)

func NewCertRotateCommand() cli.Command {
	cmd := k3sCertBase
	configfilearg.DefaultParser.ValidFlags[cmd.Name] = cmd.Flags
	return cmd
}

func CertificateRotationRun(clx *cli.Context) error {
	dataDir := clx.String("data-dir")
	if dataDir == "" {
		dataDir = rke2Path
	}
	if err := ioutil.WriteFile(rke2.ForceRestartFile(dataDir), []byte{}, 0600); err != nil {
		return err
	}
	return cert.Run(clx)
}
