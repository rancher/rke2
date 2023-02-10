package cmds

import "testing"

// Test_NewCommands confirms that all the top-level commands can be created
// successfully without causing any panics in mustCmdFromK3S.  Covering this
// with a test allows us to catch K3s flag option mismatches in testing,
// instead of not noticing until the main command crashes in functional tests.
func Test_NewCommands(t *testing.T) {
	NewServerCommand()
	NewAgentCommand()
	NewEtcdSnapshotCommand()
	NewCertRotateCommand()
	NewSecretsEncryptCommand()
}
