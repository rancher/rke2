package cmds

import (
	"io/ioutil"

	"github.com/rancher/k3s/pkg/cli/cert"
	"github.com/rancher/k3s/pkg/cli/cmds"
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
	return k3sCertBase
}

func CertificateRotationRun(clx *cli.Context) error {
	if err := ioutil.WriteFile(rke2.ForceRestartFile(clx.String("data-dir")), []byte{}, 0600); err != nil {
		return err
	}
	return cert.Run(clx)
}
