package cmds

import (
	"fmt"
	"os"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli/v2"
)

var (
	bashScript = `#! /bin/bash
_cli_bash_autocomplete() {
if [[ "${COMP_WORDS[0]}" != "source" ]]; then
	local cur opts base
	COMPREPLY=()
	cur="${COMP_WORDS[COMP_CWORD]}"
	if [[ "$cur" == "-"* ]]; then
	opts=$( ${COMP_WORDS[@]:0:$COMP_CWORD} ${cur} --generate-bash-completion )
	else
	opts=$( ${COMP_WORDS[@]:0:$COMP_CWORD} --generate-bash-completion )
	fi
	COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
	return 0
fi
}

complete -o bashdefault -o default -o nospace -F _cli_bash_autocomplete %s
`

	zshScript = `#compdef %[1]s
_cli_zsh_autocomplete() {

	local -a opts
	local cur
	cur=${words[-1]}
	if [[ "$cur" == "-"* ]]; then
	opts=("${(@f)$(_CLI_ZSH_AUTOCOMPLETE_HACK=1 ${words[@]:0:#words[@]-1} ${cur} --generate-bash-completion)}")
	else
	opts=("${(@f)$(_CLI_ZSH_AUTOCOMPLETE_HACK=1 ${words[@]:0:#words[@]-1} --generate-bash-completion)}")
	fi

	if [[ "${opts[1]}" != "" ]]; then
	_describe 'values' opts
	else
	_files
	fi

	return
}

compdef _cli_zsh_autocomplete %[1]s
`

	completionFlags = []cli.Flag{
		&cli.BoolFlag{
			Name:  "kubectl",
			Usage: "(kubectl) export kubeconfig",
		},
		&cli.BoolFlag{
			Name:  "crictl",
			Usage: "(crictl) export crictl config file",
		},
		&cli.BoolFlag{
			Name:  "ctr",
			Usage: "(ctr) export containerd sock file and set namespace",
		},
	}

	k3sCompletionBase = mustCmdFromK3S(cmds.NewCompletionCommand(Bash, Zsh), K3SFlagSet{})
)

func NewCompletionCommand() *cli.Command {
	cmd := k3sCompletionBase
	for _, command := range cmd.Subcommands {
		command.Flags = append(command.Flags, completionFlags...)
	}

	return cmd
}

func isKubectlSet(kubectl bool) string {
	if kubectl {
		return " --kubectl"
	}
	return ""
}

func isCrictlSet(crictl bool) string {
	if crictl {
		return " --crictl"
	}
	return ""
}

func isCtrSet(ctr bool) string {
	if ctr {
		return " --ctr"
	}
	return ""
}

func Bash(ctx *cli.Context) error {
	completetionScript := genCompletionScript(bashScript, ctx.Bool("kubectl"), ctx.Bool("crictl"), ctx.Bool("ctr"))
	if ctx.Bool("i") {
		return writeToRC("bash", "/.bashrc", ctx.Bool("kubectl"), ctx.Bool("crictl"), ctx.Bool("ctr"))
	}
	fmt.Println(completetionScript)
	return nil
}

func Zsh(ctx *cli.Context) error {
	completetionScript := genCompletionScript(zshScript, ctx.Bool("kubectl"), ctx.Bool("crictl"), ctx.Bool("ctr"))
	if ctx.Bool("i") {
		return writeToRC("zsh", "/.zshrc", ctx.Bool("kubectl"), ctx.Bool("crictl"), ctx.Bool("ctr"))
	}
	fmt.Println(completetionScript)
	return nil
}

func genCompletionScript(script string, kubectl, crictl, ctr bool) string {
	var completionScript string
	completionScript = fmt.Sprintf(script, version.Program)

	if kubectl {
		completionScript = fmt.Sprintf(`%s
export KUBECONFIG=/etc/rancher/rke2/rke2.yaml
    `, completionScript)
	}

	if crictl {
		completionScript = fmt.Sprintf(`%s
export CRI_CONFIG_FILE=/var/lib/rancher/rke2/agent/etc/crictl.yaml
    `, completionScript)
	}

	if ctr {
		completionScript = fmt.Sprintf(`%s
export CONTAINERD_ADDRESS=/run/k3s/containerd/containerd.sock
export CONTAINERD_NAMESPACE=k8s.io
    `, completionScript)
	}

	return completionScript
}

func writeToRC(shell, fileName string, kubectl, crictl, ctr bool) error {
	rcFileName := fileName

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	rcFileName = home + rcFileName
	f, err := os.OpenFile(rcFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	bashEntry := fmt.Sprintf("# >> %[1]s command completion (start)\n. <(%[1]s completion %[2]s%[3]s%[4]s%[5]s)\n# >> %[1]s command completion (end)", version.Program, shell, isKubectlSet(kubectl), isCrictlSet(crictl), isCtrSet(ctr))
	if _, err := f.WriteString(bashEntry); err != nil {
		return err
	}
	fmt.Printf("Autocomplete for %s added to: %s\n", shell, rcFileName)
	return nil
}
