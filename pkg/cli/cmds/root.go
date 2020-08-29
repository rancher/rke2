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
	"github.com/rancher/spur/cli"
	"github.com/sirupsen/logrus"
)

var (
	debug      bool
	profile    string
	appName    = filepath.Base(os.Args[0])
	commonFlag = []cli.Flag{
		&cli.StringFlag{
			Name:        "repo",
			Usage:       "(image) Image repository override for for RKE2 images",
			EnvVars:     []string{"RKE2_REPO"},
			Destination: &config.Repo,
		},
		&cli.StringFlag{
			Name:        "cloud-provider-name",
			Usage:       "(cloud provider) Cloud provider name",
			EnvVars:     []string{"RKE2_CLOUD_PROVIDER_NAME"},
			Destination: &config.CloudProviderName,
		},
		&cli.StringFlag{
			Name:        "cloud-provider-config",
			Usage:       "(cloud provider) Cloud provider configuration file path",
			EnvVars:     []string{"RKE2_CLOUD_PROVIDER_CONFIG"},
			Destination: &config.CloudProviderConfig,
		},
	}
)

func init() {
	// hack - force "file,dns" lookup order if go dns is used
	if os.Getenv("RES_OPTIONS") == "" {
		os.Setenv("RES_OPTIONS", " ")
	}
}

// kernelRuntimeParameters contains the names and values
// of the expected values from the Rancher Hardening guide
// for CIS 1.5 compliance.
//
// vm.panic_on_oom=0
// kernel.panic=10
// kernel.panic_on_oops=1
// kernel.keys.root_maxbytes=25000000
var kernelRuntimeParameters = map[string]int{
	"vm.overcommit_memory":      1,
	"vm.panic_on_oom":           0,
	"kernel.panic":              10,
	"kernel.panic_on_oops":      1,
	"kernel.keys.root_maxbytes": 25000000,
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

func NewApp() *cli.App {
	app := cli.NewApp()
	app.Name = appName
	app.Usage = "Rancher Kubernetes Engine 2"
	app.Version = fmt.Sprintf("%s (%s)", version.Version, version.GitCommit)
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s version %s\n", app.Name, app.Version)
	}
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:        "debug",
			Usage:       "Turn on debug logs",
			Destination: &debug,
			EnvVars:     []string{"RKE2_DEBUG"},
		},
		&cli.StringFlag{
			Name:        "profile",
			Usage:       "Indicate we need to run in CIS 1.5 mode",
			Destination: &profile,
			EnvVars:     []string{"RKE2_CIS_PROFILE"},
		},
	}

	app.Before = func(clx *cli.Context) error {
		if debug {
			logrus.SetLevel(logrus.DebugLevel)
		}
		switch profile {
		case "cis-1.5":
			if err := validateCISreqs(); err != nil {
				logrus.Fatal(err)
			}
		case "":
			// continue. warning output another layer down.
		default:
			logrus.Fatal("invalid value provided for --profile flag")
		}
		return nil
	}

	return app
}
