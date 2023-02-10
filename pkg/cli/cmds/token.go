package cmds

import (
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/cli/token"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/urfave/cli"
)

func NewTokenCommand() cli.Command {
	k3sOpts := K3SFlagSet{
		"kubeconfig": copy,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: rke2Path,
		},
	}
	subCommandOpts := map[string]K3SFlagSet{
		"create": {
			"description": copy,
			"groups":      copy,
			"ttl":         copy,
			"usages":      copy,
		},
		"list": {
			"output": copy,
		},
	}

	command := cmds.NewTokenCommands(token.Create, token.Delete, token.Generate, token.List)
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
