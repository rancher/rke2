package customflag

import (
	"fmt"
	"strconv"
	"strings"
)

var (
	UpgradeVersionSUC  UpgradeVersion
	InstallType        InstallTypeValue
	InstallUpgradeFlag MultiValueFlag
	TestCase           TestConfigFlag
)

// UpgradeVersion is a custom type to use upgradeVersionSUC flag
type UpgradeVersion struct {
	Version string
}

// InstallTypeValue is a customflag type that can be used to parse the installation type
type InstallTypeValue struct {
	Version string
	Commit  string
}

// TestConfigFlag is a customflag type that can be used to parse the test case argument
type TestConfigFlag struct {
	TestFuncName   string
	TestFunc       TestCaseFlagType
	DeployWorkload bool
}

// TestCaseFlagType is a customflag type that can be used to parse the test case argument
type TestCaseFlagType func(deployWorkload bool)

// MultiValueFlag is a customflag type that can be used to parse multiple values
type MultiValueFlag []string

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
	return fmt.Sprintf("TestFuncName: %s, DeployWorkload: %t", t.TestFuncName, t.DeployWorkload)
}

// Set parses the customflag value for TestConfigFlag
func (t *TestConfigFlag) Set(value string) error {
	parts := strings.Split(value, ",")

	if len(parts) < 1 {
		return fmt.Errorf("invalid test case customflag format")
	}

	t.TestFuncName = parts[0]
	if len(parts) > 1 {
		deployWorkload, err := strconv.ParseBool(parts[1])
		if err != nil {
			return fmt.Errorf("invalid deploy workload customflag: %v", err)
		}
		t.DeployWorkload = deployWorkload
	}

	return nil
}

// String returns the string representation of the InstallTypeValue
func (i *InstallTypeValue) String() string {
	return fmt.Sprintf("Version: %s, Commit: %s", i.Version, i.Commit)
}

// Set parses the input string and sets the Version or Commit field using Set customflag interface
func (i *InstallTypeValue) Set(value string) error {
	parts := strings.Split(value, "=")

	if len(parts) == 2 {
		switch parts[0] {
		case "INSTALL_RKE2_VERSION":
			i.Version = parts[1]
		case "INSTALL_RKE2_COMMIT":
			i.Commit = parts[1]
		default:
			return fmt.Errorf("invalid install type: %s", parts[0])
		}
	} else {
		return fmt.Errorf("invalid input format")
	}

	return nil
}

// String returns the string representation of the UpgradeVersion for SUC upgrade
func (t *UpgradeVersion) String() string {
	return t.Version
}

// Set parses the input string and sets the Version field for SUC upgrades
func (t *UpgradeVersion) Set(value string) error {
	if strings.HasPrefix(value, "v") && strings.HasSuffix(value, "rke2r1") {
		t.Version = value
	} else {
		return fmt.Errorf("invalid install format: %s", value)
	}

	return nil
}
