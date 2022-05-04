package cmds

import (
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/secretsencrypt"
	"github.com/urfave/cli"
)

var secretsEncryptSubcommands = []cli.Command{
	{
		Name:            "status",
		Usage:           "Print current status of secrets encryption",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          secretsencrypt.Status,
		Flags: append(cmds.EncryptFlags, &cli.StringFlag{
			Name:        "output,o",
			Usage:       "Status format. Default: text. Optional: json",
			Destination: &cmds.ServerConfig.EncryptOutput,
		}),
	},
	{
		Name:            "enable",
		Usage:           "Enable secrets encryption",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          secretsencrypt.Enable,
		Flags:           cmds.EncryptFlags,
	},
	{
		Name:            "disable",
		Usage:           "Disable secrets encryption",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          secretsencrypt.Disable,
		Flags:           cmds.EncryptFlags,
	},
	{
		Name:            "prepare",
		Usage:           "Prepare for encryption keys rotation",
		SkipFlagParsing: false,
		SkipArgReorder:  true,
		Action:          secretsencrypt.Prepare,
		Flags: append(cmds.EncryptFlags, &cli.BoolFlag{
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
		Flags: append(cmds.EncryptFlags, &cli.BoolFlag{
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
		Flags: append(cmds.EncryptFlags,
			&cli.BoolFlag{
				Name:        "f, force",
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

func NewSecretsEncryptCommand() cli.Command {

	var modifiedSubcommands []cli.Command
	for _, subcommand := range secretsEncryptSubcommands {
		modifiedSubcommands = append(modifiedSubcommands, mustCmdFromK3S(subcommand, map[string]*K3SFlagOption{
			"data-dir": copy,
			"token":    copy,
			"server": {
				Default: "https://127.0.0.1:9345",
			},
			"f":      ignore,
			"skip":   ignore,
			"output": ignore,
		}))
	}
	return cmds.NewSecretsEncryptCommand(cli.ShowAppHelp, modifiedSubcommands)
}
