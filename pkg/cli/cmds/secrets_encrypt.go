package cmds

import (
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/secretsencrypt"
	"github.com/urfave/cli"
)

func NewSecretsEncryptCommand() cli.Command {
	k3sOpts := K3SFlagSet{
		"data-dir": copyFlag,
		"token":    copyFlag,
		"server": {
			Default: "https://127.0.0.1:9345",
		},
	}
	subcommandOpts := map[string]K3SFlagSet{
		"status": {
			"output": copyFlag,
		},
		"prepare": {
			"force": copyFlag,
		},
		"rotate": {
			"force": copyFlag,
		},
		"reencrypt": {
			"force": copyFlag,
			"skip":  copyFlag,
		},
	}

	command := cmds.NewSecretsEncryptCommands(
		secretsencrypt.Status,
		secretsencrypt.Enable,
		secretsencrypt.Disable,
		secretsencrypt.Prepare,
		secretsencrypt.Rotate,
		secretsencrypt.Reencrypt,
		secretsencrypt.RotateKeys)

	for i, subcommand := range command.Subcommands {
		if s, ok := subcommandOpts[subcommand.Name]; ok {
			k3sOpts.CopyInto(s)
			command.Subcommands[i] = mustCmdFromK3S(subcommand, s)
		} else {
			command.Subcommands[i] = mustCmdFromK3S(subcommand, k3sOpts)
		}
	}
	return mustCmdFromK3S(command, nil)
}
