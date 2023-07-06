package customflag

import (
	"fmt"
	"strconv"
	"strings"
)

var ServiceFlag FlagConfig
var TestCaseNameFlag StringSlice

type FlagConfig struct {
	InstallType       InstallTypeValueFlag
	InstallUpgrade    MultiValueFlag
	TestConfig        TestConfigFlag
	ClusterConfig     ClusterConfigFlag
	UpgradeVersionSUC UpgradeVersionFlag
}

// UpgradeVersionFlag is a custom type to use upgradeVersionSUC flag
type UpgradeVersionFlag struct {
	Version string
}

// InstallTypeValueFlag is a customflag type that can be used to parse the installation type
type InstallTypeValueFlag struct {
	Version []string
	Commit  []string
	Channel string
}

// TestConfigFlag is a customflag type that can be used to parse the test case argument
type TestConfigFlag struct {
	TestFuncNames  []string
	TestFuncs      []TestCaseFlag
	DeployWorkload bool
	WorkloadName   string
	Description    string
}

// TestCaseFlag is a customflag type that can be used to parse the test case argument
type TestCaseFlag func(deployWorkload bool)

// MultiValueFlag is a customflag type that can be used to parse multiple values
type MultiValueFlag []string

// DestroyFlag is a customflag type that can be used to parse the destroy flag
type DestroyFlag bool

// ClusterConfigFlag is a customFlag type that can be used to change some cluster config
type ClusterConfigFlag struct {
	Destroy DestroyFlag
}

// StringSlice defines a custom flag type for string slice
type StringSlice []string

// String returns the string representation of the StringSlice
func (s *StringSlice) String() string {
	return strings.Join(*s, ",")
}

// Set parses the input string and sets the StringSlice using Set customflag interface
func (s *StringSlice) Set(value string) error {
	*s = strings.Split(value, ",")
	return nil
}

// String returns the string representation of the MultiValueFlag
func (m *MultiValueFlag) String() string {
	return strings.Join(*m, ",")
}

// Set func sets multiValueFlag appending the value
func (m *MultiValueFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

// String returns the string representation of the TestConfigFlag
func (t *TestConfigFlag) String() string {
	return fmt.Sprintf("TestFuncName: %s", t.TestFuncNames)
}

// Set parses the customFlag value for TestConfigFlag
func (t *TestConfigFlag) Set(value string) error {
	t.TestFuncNames = strings.Split(value, ",")
	return nil
}

// String returns the string representation of the InstallTypeValue
func (i *InstallTypeValueFlag) String() string {
	return fmt.Sprintf("Version: %s, Commit: %s", i.Version, i.Commit)
}

// Set parses the input string and sets the Version or Commit field using Set customflag interface
func (i *InstallTypeValueFlag) Set(value string) error {
	parts := strings.Split(value, "=")

	for _, part := range parts {
		subParts := strings.Split(part, "=")
		if len(subParts) != 2 {
			return fmt.Errorf("invalid input format")
		}
		switch parts[0] {
		case "INSTALL_RKE2_VERSION":
			i.Version = append(i.Version, subParts[1])
		case "INSTALL_RKE2_COMMIT":
			i.Commit = append(i.Commit, subParts[1])
		default:
			return fmt.Errorf("invalid install type: %s", parts[0])
		}
	}

	return nil
}

// String returns the string representation of the UpgradeVersion for SUC upgrade
func (t *UpgradeVersionFlag) String() string {
	return t.Version
}

// Set parses the input string and sets the Version field for SUC upgrades
func (t *UpgradeVersionFlag) Set(value string) error {
	if !strings.HasPrefix(value, "v") && !strings.HasSuffix(value, "rke2r1") {
		return fmt.Errorf("invalid install format: %s", value)
	}

	t.Version = value
	return nil
}

// String returns the string representation of the DestroyFlag
func (d *DestroyFlag) String() string {
	return fmt.Sprintf("%v", *d)
}

// Set parses the customFlag value for DestroyFlag
func (d *DestroyFlag) Set(value string) error {
	v, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	*d = DestroyFlag(v)

	return nil
}
