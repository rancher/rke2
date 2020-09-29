package cmds

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	debug      bool
	profile    string
	appName    = filepath.Base(os.Args[0])
	commonFlag = []cli.Flag{
		&cli.StringFlag{
			Name:        "system-default-registry",
			Usage:       "(image) Private registry to be used for all system Docker images",
			EnvVar:      "RKE2_SYSTEM_DEFAULT_REGISTRY",
			Destination: &config.SystemDefaultRegistry,
		},
		&cli.StringFlag{
			Name:        "cloud-provider-name",
			Usage:       "(cloud provider) Cloud provider name",
			EnvVar:      "RKE2_CLOUD_PROVIDER_NAME",
			Destination: &config.CloudProviderName,
		},
		&cli.StringFlag{
			Name:        "cloud-provider-config",
			Usage:       "(cloud provider) Cloud provider configuration file path",
			EnvVar:      "RKE2_CLOUD_PROVIDER_CONFIG",
			Destination: &config.CloudProviderConfig,
		},
		&cli.StringFlag{
			Name:        "profile",
			Usage:       "(security) Validate system configuration against the selected benchmark (valid items: cis-1.5)",
			EnvVar:      "RKE2_CIS_PROFILE",
			Destination: &profile,
		},
	}
)

const pkdFlagName = "protect-kernel-defaults"

func init() {
	// hack - force "file,dns" lookup order if go dns is used
	if os.Getenv("RES_OPTIONS") == "" {
		os.Setenv("RES_OPTIONS", " ")
	}
}

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

// validateCISreqs checks if the system is in compliance
// with CIS 1.5 benchmark requirements.
func validateCISreqs() error {
	ce := make(cisErrors, 0)

	// get nonexistent user information
	if _, err := user.Lookup("etcd"); err != nil {
		ce = append(ce, err)
	}
	for kp, pv := range kernelRuntimeParameters {
		cv, err := sysctl(kp)
		if err != nil {
			// failing since we can't process
			// further whether the flag was
			// called or not.
			logrus.Fatal(err)
		}
		if cv != pv {
			ce = append(ce, fmt.Errorf("%s=%d - expected %d", kp, cv, pv))
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
	if clx.IsSet(pkdFlagName) && !clx.Bool(pkdFlagName) {
		return fmt.Errorf("--%s must be true when using --profile=%s", pkdFlagName, profile)
	}
	return clx.Set(pkdFlagName, "true")
}

func NewApp() *cli.App {
	app := cli.NewApp()
	app.Name = appName
	app.Usage = "Rancher Kubernetes Engine 2"
	app.Version = fmt.Sprintf("%s (%s)", version.Version, version.GitCommit)
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s version %s\n", app.Name, app.Version)
	}
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "Turn on debug logs",
			Destination: &debug,
			EnvVar:      "RKE2_DEBUG",
		},
	}

	app.Before = func(clx *cli.Context) error {
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}

	return app
}
