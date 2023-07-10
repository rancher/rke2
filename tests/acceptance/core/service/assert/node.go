package assert

import (
	"fmt"
	"strings"

	"github.com/rancher/rke2/tests/acceptance/core/service/customflag"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type NodeAssertFunc func(g Gomega, node shared.Node)

// NodeAssertVersionTypeUpgrade  custom assertion func that asserts that node version is as expected
func NodeAssertVersionTypeUpgrade(installType customflag.FlagConfig) NodeAssertFunc {
	if installType.InstallUpgrade != nil {
		if strings.HasPrefix(customflag.ServiceFlag.InstallUpgrade.String(), "v") {
			return assertVersion(installType)
		}
		return assertCommit(installType)
	}

	return func(g Gomega, node shared.Node) {
		GinkgoT().Errorf("no version or commit specified for upgrade assertion")
	}
}

// assertVersion returns the NodeAssertFunc for asserting version
func assertVersion(installType customflag.FlagConfig) NodeAssertFunc {
	if installType.InstallUpgrade != nil {
		if strings.HasPrefix(customflag.ServiceFlag.InstallUpgrade.String(), "v") {
			fmt.Printf("Asserting Version: %s\n", installType.InstallUpgrade.String())
			return func(g Gomega, node shared.Node) {
				g.Expect(node.Version).Should(ContainSubstring(installType.InstallUpgrade.String()),
					"Nodes should all be upgraded to the specified version", node.Name)
			}
		}
	}

	return func(g Gomega, node shared.Node) {
		GinkgoT().Errorf("no version specified for upgrade assertion")
	}
}

// assertCommit returns the NodeAssertFunc for asserting commit
func assertCommit(installType customflag.FlagConfig) NodeAssertFunc {
	if installType.InstallUpgrade != nil {
		upgradedVersion := shared.GetRke2Version()
		fmt.Printf("Asserting Commit: %s\n Version: %s",
			installType.InstallUpgrade.String(), upgradedVersion)
		return func(g Gomega, node shared.Node) {
			g.Expect(upgradedVersion).Should(ContainSubstring(installType.InstallUpgrade.String()),
				"Nodes should all be upgraded to the specified commit", node.Name)
		}
	}

	return func(g Gomega, node shared.Node) {
		GinkgoT().Errorf("no commit specified for upgrade validation")
	}
}

// NodeAssertVersionUpgraded custom assertion func that asserts that node version is as expected
func NodeAssertVersionUpgraded() NodeAssertFunc {
	return func(g Gomega, node shared.Node) {
		g.Expect(&customflag.ServiceFlag.UpgradeVersionSUC).Should(ContainSubstring(node.Version),
			"Nodes should all be upgraded to the specified version", node.Name)
	}
}

// NodeAssertReadyStatus custom assertion func that asserts that the node is in Ready state.
func NodeAssertReadyStatus() NodeAssertFunc {
	return func(g Gomega, node shared.Node) {
		g.Expect(node.Status).Should(Equal("Ready"),
			"Nodes should all be in Ready state")
	}
}

// CheckComponentCmdNode runs a command on a node and asserts that the value received
// contains the specified substring.
func CheckComponentCmdNode(cmd, assert, ip string) {
	Eventually(func(g Gomega) {
		fmt.Println("Executing cmd: ", cmd)
		res, err := shared.RunCommandOnNode(cmd, ip)
		if err != nil {
			return
		}
		g.Expect(res).Should(ContainSubstring(assert))
		fmt.Println("Result:", res+"\nMatched with assert:\n", assert)
	}, "420s", "3s").Should(Succeed())
}
