package template

import (
	"fmt"
	"strings"

	"github.com/rancher/rke2/tests/acceptance/core/service/customflag"
	"github.com/rancher/rke2/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
)

// VersionTemplate is a template for testing RKE2 versions + test cases and upgrading cluster if needed
func VersionTemplate(test VersionTestTemplate) {
	if customflag.ServiceFlag.TestConfig.WorkloadName != "" &&
		strings.HasSuffix(customflag.ServiceFlag.TestConfig.WorkloadName, ".yaml") {
		_, err := shared.ManageWorkload(
			"create",
			customflag.ServiceFlag.TestConfig.WorkloadName,
		)
		if err != nil {
			GinkgoT().Errorf(err.Error())
			return
		}
	}

	err := checkVersion(test)
	if err != nil {
		GinkgoT().Errorf(err.Error())
		return
	}

	if test.InstallUpgrade != nil {
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
}
