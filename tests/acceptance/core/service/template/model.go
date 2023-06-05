package template

var TestMapFlag TestMap

// VersionTestTemplate represents a version test scenario with test configurations and commands.
type VersionTestTemplate struct {
	Description     string
	TestCombination *RunCmd
	InstallUpgrade  []string
	TestConfig      *TestConfig
}

// RunCmd represents the command sets to run on host and node.
type RunCmd struct {
	RunOnHost []TestMap
	RunOnNode []TestMap
}

// TestMap represents a single test command with key:value pairs.
type TestMap struct {
	Cmd                       string
	ExpectedValue             string
	ExpectedValueUpgrade      string
	ExpectedValueUpgradedHost string
	ExpectedValueUpgradedNode string
	CmdHost                   string
	ExpectedValueHost         string
	CmdNode                   string
	ExpectedValueNode         string
	Description               string
}

// TestConfig represents the testcase function configuration
type TestConfig struct {
	TestFunc       TestCase
	DeployWorkload bool
}

// TestCase is a custom type representing the test function.
type TestCase func(deployWorkload bool)

// TestCaseWrapper wraps a test function and calls it with the given VersionTestTemplate.
func TestCaseWrapper(v VersionTestTemplate) {
	v.TestConfig.TestFunc(v.TestConfig.DeployWorkload)
}
