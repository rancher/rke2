package cmds

import (
	"testing"

	"github.com/urfave/cli"
)

// Test_NewCommands confirms that all the top-level commands can be created
// successfully without causing any panics in mustCmdFromK3S.  Covering this
// with a test allows us to catch K3s flag option mismatches in testing,
// instead of not noticing until the main command crashes in functional tests.
func Test_NewCommands(t *testing.T) {
	app := cli.NewApp()
	app.Name = "rke2"
	app.Commands = []cli.Command{
		NewServerCommand(),
		NewAgentCommand(),
		NewEtcdSnapshotCommand(),
		NewCertCommand(),
		NewSecretsEncryptCommand(),
		NewTokenCommand(),
		NewCompletionCommand(),
	}

	for _, command := range app.Commands {
		t.Logf("Testing command: %s", command.Name)
		app.Run([]string{app.Name, command.Name, "--help"})

		for _, subcommand := range command.Subcommands {
			t.Logf("Testing subcommand: %s %s", command.Name, subcommand.Name)
			app.Run([]string{app.Name, command.Name, subcommand.Name, "--help"})
		}
	}
}
