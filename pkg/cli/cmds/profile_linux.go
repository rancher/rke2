//go:build !windows
// +build !windows

package cmds

import (
	"fmt"
	"io/ioutil"
	"os/user"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	protectKernelDefaultsFlagName = "protect-kernel-defaults"
)

// kernelRuntimeParameters contains the names and values
// of the expected values from the Rancher Hardening guide
// for CIS 1.5 compliance.
var kernelRuntimeParameters = map[string]int{
	"vm.overcommit_memory": 1,
	"vm.panic_on_oom":      0,
	"kernel.panic":         10,
	"kernel.panic_on_oops": 1,
}

// sysctl retrieves the value of the given sysctl.
func sysctl(s string) (int, error) {
	s = strings.ReplaceAll(s, ".", "/")
	v, err := ioutil.ReadFile("/proc/sys/" + s)
	if err != nil {
		return 0, err
	}
	if len(v) < 2 || v[len(v)-1] != '\n' {
		return 0, fmt.Errorf("invalid contents: %s", s)
	}
	return strconv.Atoi(strings.Replace(string(v), "\n", "", -1))
}

// cisErrors holds errors reported during
// the start-up routine that checks for
// CIS compliance.
type cisErrors []error

// Error provides a string representation of the
// cisErrors type and satisfies the Error interface.
func (c cisErrors) Error() string {
	var err strings.Builder
	for _, e := range c {
		err.WriteString(e.Error() + "\n")
	}
	return err.String()
}

// validateCISReqs checks if the system is in compliance
// with CIS 1.5 benchmark requirements. The nodeType string
// is used to filter out tests that may only be relevant to servers
// or agents.
func validateCISReqs(role CLIRole) error {
	ce := make(cisErrors, 0)

	// etcd user only needs to exist on servers
	if role == Server {
		if _, err := user.Lookup("etcd"); err != nil {
			ce = append(ce, errors.Wrap(err, "missing required"))
		}
		if _, err := user.LookupGroup("etcd"); err != nil {
			ce = append(ce, errors.Wrap(err, "missing required"))
		}
	}

	for kp, pv := range kernelRuntimeParameters {
		cv, err := sysctl(kp)
		if err != nil {
			// Fail immediately if we cannot retrieve the current value,
			// since it is unlikely that we will be able to retrieve others
			// if this one failed.
			logrus.Fatal(err)
		}
		if cv != pv {
			ce = append(ce, fmt.Errorf("invalid kernel parameter value %s=%d - expected %d", kp, cv, pv))
		}
	}
	if len(ce) != 0 {
		return ce
	}
	return nil
}

// setCISFlags validates and sets any CLI flags necessary to ensure
// compliance with the profile.
func setCISFlags(clx *cli.Context) error {
	// If the user has specifically set this to false, raise an error
	if clx.IsSet(protectKernelDefaultsFlagName) && !clx.Bool(protectKernelDefaultsFlagName) {
		return fmt.Errorf("--%s must be true when using --profile=%s", protectKernelDefaultsFlagName, clx.String("profile"))
	}
	return clx.Set(protectKernelDefaultsFlagName, "true")
}

func validateProfile(clx *cli.Context, role CLIRole) {
	switch clx.String("profile") {
	case rke2.CISProfile123, rke2.CISProfile:
		if err := validateCISReqs(role); err != nil {
			logrus.Fatal(err)
		}
		if err := setCISFlags(clx); err != nil {
			logrus.Fatal(err)
		}
	case "":
		logrus.Warn("not running in CIS mode")
	default:
		logrus.Fatal("invalid value provided for --profile flag")
	}
}
