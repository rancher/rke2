package template

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
)

// VersionTemplate is a template for testing RKE2 versions + test cases and upgrading cluster if needed
func VersionTemplate(test VersionTestTemplate) {
	err := checkVersion(test)
	if err != nil {
		GinkgoT().Errorf(err.Error())
		return
	}

	for _, version := range test.InstallUpgrade {
		if GinkgoT().Failed() {
			fmt.Println("checkVersion failed, upgrade not performed")
			return
		}

		err = upgradeVersion(test, version)
		if err != nil {
			GinkgoT().Errorf("error upgrading: %v\n", err)
			return
		}

		err = checkVersion(test)
		if err != nil {
			GinkgoT().Errorf(err.Error())
			return
		}

		if test.TestConfig != nil {
			TestCaseWrapper(test)
		}
	}
}
