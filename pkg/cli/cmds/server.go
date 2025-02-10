package cmds

import (
	"errors"
	"strings"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/rancher/rke2/pkg/rke2"
	"github.com/rancher/wrangler/v3/pkg/slice"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	rke2Path = "/var/lib/rancher/rke2"
)

var (
	config = rke2.Config{}

	serverFlag = []cli.Flag{
		rke2.CNIFlag,
		rke2.IngressControllerFlag,
		rke2.ServiceLBFlag,
	}

	k3sServerBase = mustCmdFromK3S(cmds.NewServerCommand(ServerRun), K3SFlagSet{
		"config":                 copyFlag,
		"debug":                  copyFlag,
		"v":                      hideFlag,
		"vmodule":                hideFlag,
		"log":                    hideFlag,
		"alsologtostderr":        hideFlag,
		"bind-address":           copyFlag,
		"https-listen-port":      dropFlag,
		"supervisor-port":        dropFlag,
		"apiserver-port":         dropFlag,
		"apiserver-bind-address": dropFlag,
		"advertise-address":      copyFlag,
		"advertise-port":         dropFlag,
		"tls-san":                copyFlag,
		"tls-san-security":       copyFlag,
		"data-dir": {
			Usage:   "(data) Folder to hold state",
			Default: rke2Path,
		},
		"disable-agent":                     hideFlag,
		"cluster-cidr":                      copyFlag,
		"service-cidr":                      copyFlag,
		"cluster-init":                      dropFlag,
		"cluster-reset":                     copyFlag,
		"cluster-reset-restore-path":        copyFlag,
		"cluster-dns":                       copyFlag,
		"cluster-domain":                    copyFlag,
		"flannel-backend":                   dropFlag,
		"vpn-auth":                          dropFlag,
		"vpn-auth-file":                     dropFlag,
		"token":                             copyFlag,
		"token-file":                        copyFlag,
		"write-kubeconfig":                  copyFlag,
		"write-kubeconfig-mode":             copyFlag,
		"write-kubeconfig-group":            copyFlag,
		"kube-apiserver-arg":                copyFlag,
		"etcd-arg":                          copyFlag,
		"kube-scheduler-arg":                copyFlag,
		"kube-controller-arg":               dropFlag, // deprecated version of kube-controller-manager-arg
		"kube-controller-manager-arg":       copyFlag,
		"kube-cloud-controller-manager-arg": copyFlag,
		"kube-cloud-controller-arg":         dropFlag, // deprecated version of kube-cloud-controller-manager-arg
		"datastore-endpoint":                copyFlag,
		"datastore-cafile":                  copyFlag,
		"datastore-certfile":                copyFlag,
		"datastore-keyfile":                 copyFlag,
		"kine-tls":                          dropFlag,
		"default-local-storage-path":        dropFlag,
		"disable": {
			Usage: "(components) Do not deploy packaged components and delete any deployed components (valid items: " + strings.Join(rke2.DisableItems, ", ") + ")",
		},
		"disable-scheduler":                 copyFlag,
		"disable-cloud-controller":          copyFlag,
		"disable-network-policy":            dropFlag,
		"disable-kube-proxy":                copyFlag,
		"disable-apiserver":                 copyFlag,
		"disable-controller-manager":        copyFlag,
		"disable-etcd":                      copyFlag,
		"etcd-disable-snapshots":            copyFlag,
		"etcd-snapshot-schedule-cron":       copyFlag,
		"etcd-snapshot-retention":           copyFlag,
		"etcd-snapshot-dir":                 copyFlag,
		"etcd-snapshot-name":                copyFlag,
		"etcd-snapshot-compress":            copyFlag,
		"node-name":                         copyFlag,
		"with-node-id":                      copyFlag,
		"node-label":                        copyFlag,
		"node-taint":                        copyFlag,
		"image-credential-provider-bin-dir": copyFlag,
		"image-credential-provider-config":  copyFlag,
		"docker":                            dropFlag,
		"container-runtime-endpoint":        copyFlag,
		"disable-default-registry-endpoint": copyFlag,
		"nonroot-devices":                   copyFlag,
		"embedded-registry":                 copyFlag,
		"supervisor-metrics":                copyFlag,
		"image-service-endpoint":            dropFlag,
		"pause-image":                       dropFlag,
		"default-runtime":                   copyFlag,
		"private-registry":                  copyFlag,
		"system-default-registry":           copyFlag,
		"node-ip":                           copyFlag,
		"node-external-ip":                  copyFlag,
		"node-internal-dns":                 copyFlag,
		"node-external-dns":                 copyFlag,
		"resolv-conf":                       copyFlag,
		"flannel-iface":                     dropFlag,
		"flannel-conf":                      dropFlag,
		"flannel-cni-conf":                  dropFlag,
		"flannel-ipv6-masq":                 dropFlag,
		"flannel-external-ip":               dropFlag,
		"egress-selector-mode":              copyFlag,
		"kubelet-arg":                       copyFlag,
		"kube-proxy-arg":                    copyFlag,
		"rootless":                          dropFlag,
		"prefer-bundled-bin":                dropFlag,
		"agent-token":                       copyFlag,
		"agent-token-file":                  copyFlag,
		"server":                            copyFlag,
		"secrets-encryption":                hideFlag,
		"protect-kernel-defaults":           copyFlag,
		"snapshotter":                       copyFlag,
		"selinux":                           copyFlag,
		"lb-server-port":                    copyFlag,
		"service-node-port-range":           copyFlag,
		"etcd-expose-metrics":               copyFlag,
		"airgap-extra-registry":             copyFlag,
		"etcd-s3":                           copyFlag,
		"etcd-s3-access-key":                copyFlag,
		"etcd-s3-bucket":                    copyFlag,
		"etcd-s3-config-secret":             copyFlag,
		"etcd-s3-endpoint":                  copyFlag,
		"etcd-s3-endpoint-ca":               copyFlag,
		"etcd-s3-folder":                    copyFlag,
		"etcd-s3-insecure":                  copyFlag,
		"etcd-s3-proxy":                     copyFlag,
		"etcd-s3-region":                    copyFlag,
		"etcd-s3-secret-key":                copyFlag,
		"etcd-s3-session-token":             copyFlag,
		"etcd-s3-skip-ssl-verify":           copyFlag,
		"etcd-s3-timeout":                   copyFlag,
		"disable-helm-controller":           dropFlag,
		"helm-job-image":                    copyFlag,
		"enable-pprof":                      copyFlag,
		"servicelb-namespace":               copyFlag,
	})
)

func NewServerCommand() cli.Command {
	cmd := k3sServerBase
	cmd.Flags = append(cmd.Flags, serverFlag...)
	cmd.Flags = append(cmd.Flags, commonFlag...)
	configfilearg.DefaultParser.ValidFlags[cmd.Name] = cmd.Flags
	return cmd
}

func ServerRun(clx *cli.Context) error {
	validateCloudProviderName(clx, Server)
	validateProfile(clx, Server)
	validateCNI(clx)
	validateIngress(clx)
	return rke2.Server(clx, config)
}

// validateCNI validates the CNI selection, and disables any un-selected CNI charts
func validateCNI(clx *cli.Context) {
	disableExceptSelected(clx, rke2.CNIItems, rke2.CNIFlag, func(values cli.StringSlice) (cli.StringSlice, error) {
		switch len(values) {
		case 0:
			values = append(values, "canal")
			fallthrough
		case 1:
			if values[0] == "multus" {
				return nil, errors.New("multus must be used alongside another primary cni selection")
			}
			clx.Set("disable", "rke2-multus")
		case 2:
			if values[0] == "multus" {
				values = values[1:]
			} else {
				return nil, errors.New("may only provide multiple values if multus is the first value")
			}
		default:
			return nil, errors.New("must specify 1 or 2 values")
		}
		return values, nil
	})
}

// validateCNI validates the ingress controller selection, and disables any un-selected ingress controller charts
func validateIngress(clx *cli.Context) {
	disableExceptSelected(clx, rke2.IngressItems, rke2.IngressControllerFlag, func(values cli.StringSlice) (cli.StringSlice, error) {
		if len(values) == 0 {
			values = append(values, "ingress-nginx")
		}
		return values, nil
	})
}

// disableExceptSelected takes a list of valid flag values, and a CLI StringSlice flag that holds the user's selected values.
// Selected values are split to support comma-separated lists, in addition to repeated use of the same flag.
// Once the list has been split, a validation function is called to allow for custom validation or defaulting of selected values.
// Finally, charts for any valid items not selected are added to the --disable list.
// A value of 'none' will cause all valid items to be disabled.
// Errors from the validation function, or selection of a value not in the valid list, will cause a fatal error to be logged.
func disableExceptSelected(clx *cli.Context, valid []string, flag *cli.StringSliceFlag, validateFunc func(cli.StringSlice) (cli.StringSlice, error)) {
	// split comma-separated values
	values := cli.StringSlice{}
	if flag.Value != nil {
		for _, value := range *flag.Value {
			for _, v := range strings.Split(value, ",") {
				values = append(values, v)
			}
		}
	}

	// validate the flag after splitting values
	if v, err := validateFunc(values); err != nil {
		logrus.Fatalf("Failed to validate --%s flag: %v", flag.Name, err)
	} else {
		flag.Value = &v
	}

	// prepare a list of items to disable, based on all valid components.
	// we have to use an intermediate set because the flag interface
	// doesn't allow us to remove flag values once added.
	disabledCharts := sets.Set[string]{}
	for _, d := range valid {
		disabledCharts.Insert("rke2-"+d, "rke2-"+d+"-crd")
	}

	// re-enable components for any selected flag values
	for _, d := range *flag.Value {
		switch {
		case d == "none":
			break
		case slice.ContainsString(valid, d):
			disabledCharts.Delete("rke2-"+d, "rke2-"+d+"-crd")
		default:
			logrus.Fatalf("Invalid value %s for --%s flag: must be one of %s", d, flag.Name, strings.Join(valid, ","))
		}
	}

	for _, c := range disabledCharts.UnsortedList() {
		clx.Set("disable", c)
	}
}
