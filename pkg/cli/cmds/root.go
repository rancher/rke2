package cmds

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/rancher/k3s/pkg/version"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/rancher/wrangler/pkg/slice"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var (
	debug      bool
	appName    = filepath.Base(os.Args[0])
	commonFlag = []cli.Flag{
		&cli.StringFlag{
			Name:        images.KubeAPIServer,
			Usage:       "(image) Override image to use for kube-apiserver",
			EnvVar:      "RKE2_KUBE_APISERVER_IMAGE",
			Destination: &config.Images.KubeAPIServer,
		},
		&cli.StringFlag{
			Name:        images.KubeControllerManager,
			Usage:       "(image) Override image to use for kube-controller-manager",
			EnvVar:      "RKE2_KUBE_CONTROLLER_MANAGER_IMAGE",
			Destination: &config.Images.KubeControllerManager,
		},
		&cli.StringFlag{
			Name:        images.KubeScheduler,
			Usage:       "(image) Override image to use for kube-scheduler",
			EnvVar:      "RKE2_KUBE_SCHEDULER_IMAGE",
			Destination: &config.Images.KubeScheduler,
		},
		&cli.StringFlag{
			Name:        images.Pause,
			Usage:       "(image) Override image to use for pause",
			EnvVar:      "RKE2_PAUSE_IMAGE",
			Destination: &config.Images.Pause,
		},
		&cli.StringFlag{
			Name:        images.Runtime,
			Usage:       "(image) Override image to use for runtime binaries (containerd, kubectl, crictl, etc)",
			EnvVar:      "RKE2_RUNTIME_IMAGE",
			Destination: &config.Images.Runtime,
		},
		&cli.StringFlag{
			Name:        images.ETCD,
			Usage:       "(image) Override image to use for etcd",
			EnvVar:      "RKE2_ETCD_IMAGE",
			Destination: &config.Images.ETCD,
		},
		&cli.StringFlag{
			Name:        "kubelet-path",
			Usage:       "(experimental/agent) Override kubelet binary path",
			EnvVar:      "RKE2_KUBELET_PATH",
			Destination: &config.KubeletPath,
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
			Name:   "profile",
			Usage:  "(security) Validate system configuration against the selected benchmark (valid items: " + rke2.CISProfile15 + ", " + rke2.CISProfile16 + " )",
			EnvVar: "RKE2_CIS_PROFILE",
		},
		&cli.StringFlag{
			Name:        "audit-policy-file",
			Usage:       "(security) Path to the file that defines the audit policy configuration",
			EnvVar:      "RKE2_AUDIT_POLICY_FILE",
			Destination: &config.AuditPolicyFile,
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

// validateCISReqs checks if the system is in compliance
// with CIS 1.5 benchmark requirements. The nodeType string
// is used to filter out tests that may only be relevant to servers
// or agents.
func validateCISReqs(nodeType string) error {
	ce := make(cisErrors, 0)

	// etcd user only needs to exist on servers
	if nodeType == "server" {
		if _, err := user.Lookup("etcd"); err != nil {
			ce = append(ce, fmt.Errorf("missing required %w", err))
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
	if clx.IsSet(pkdFlagName) && !clx.Bool(pkdFlagName) {
		return fmt.Errorf("--%s must be true when using --profile=%s", pkdFlagName, clx.String("profile"))
	}
	return clx.Set(pkdFlagName, "true")
}

func validateProfile(clx *cli.Context) {
	switch clx.String("profile") {
	case rke2.CISProfile15, rke2.CISProfile16:
		if err := validateCISReqs("server"); err != nil {
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

func validateCloudProviderName(clx *cli.Context) {
	cloudProvider := clx.String("cloud-provider-name")
	if cloudProvider == "vsphere" {
		clx.Set("cloud-provider-name", "external")
	} else {
		if slice.ContainsString(clx.FlagNames(), "disable") {
			clx.Set("disable", "rancher-vsphere-cpi")
			clx.Set("disable", "rancher-vsphere-csi")
		}
	}
}

func NewApp() *cli.App {
	app := cli.NewApp()
	app.Name = appName
	app.Usage = "Rancher Kubernetes Engine 2"
	app.Version = fmt.Sprintf("%s (%s)", version.Version, version.GitCommit)
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("%s version %s\n", app.Name, app.Version)
		fmt.Printf("go version %s\n", runtime.Version())
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
