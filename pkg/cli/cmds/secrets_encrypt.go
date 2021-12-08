package cmds

import (
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/cli/secretsencrypt"
	"github.com/rancher/k3s/pkg/version"
	"github.com/urfave/cli"
)

var encryptFlags = append(cmds.EncryptFlags, cli.StringFlag{
	Name:        "server,s",
	Usage:       "(cluster) Server to request from",
	EnvVar:      version.ProgramUpper + "_URL",
	Value:       "https://127.0.0.1:9345",
	Destination: &cmds.ServerConfig.ServerURL,
})

var secretsEncryptSubcommands = []cli.Command{
	{
		Name:            "status",
		Usage:           "Print current status of secrets encryption",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          secretsencrypt.Status,
		Flags:           encryptFlags,
	},
	{
		Name:            "enable",
		Usage:           "Enable secrets encryption",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          secretsencrypt.Enable,
		Flags:           encryptFlags,
	},
	{
		Name:            "disable",
		Usage:           "Disable secrets encryption",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          secretsencrypt.Disable,
		Flags:           encryptFlags,
	},
	{
		Name:            "prepare",
		Usage:           "Prepare for encryption keys rotation",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          secretsencrypt.Prepare,
		Flags: append(encryptFlags, &cli.BoolFlag{
			Name:        "f,force",
			Usage:       "Force preparation.",
			Destination: &cmds.ServerConfig.EncryptForce,
		}),
	},
	{
		Name:            "rotate",
		Usage:           "Rotate secrets encryption keys",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          secretsencrypt.Rotate,
		Flags: append(encryptFlags, &cli.BoolFlag{
			Name:        "f,force",
			Usage:       "Force key rotation.",
			Destination: &cmds.ServerConfig.EncryptForce,
		}),
	},
	{
		Name:            "reencrypt",
		Usage:           "Reencrypt all data with new encryption key",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          secretsencrypt.Reencrypt,
		Flags: append(encryptFlags,
			&cli.BoolFlag{
				Name:        "f,force",
				Usage:       "Force secrets reencryption.",
				Destination: &cmds.ServerConfig.EncryptForce,
			},
			&cli.BoolFlag{
				Name:        "skip",
				Usage:       "Skip removing old key",
				Destination: &cmds.ServerConfig.EncryptSkip,
			}),
	},
}

var (
	k3sSecretsEncryptBase = mustCmdFromK3S(cmds.NewSecretsEncryptCommand(cli.ShowAppHelp, secretsEncryptSubcommands), nil)
)

func NewSecretsEncryptCommand() cli.Command {
	return k3sSecretsEncryptBase
}
