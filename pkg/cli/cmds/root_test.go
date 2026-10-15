package cmds

import (
	"flag"
	"slices"
	"testing"

	"github.com/urfave/cli/v2"
)

func Test_UnitValidateCloudProvider(t *testing.T) {
	allProviders := []string{
		"rancher-vsphere-csi",
		"rancher-vsphere-cpi",
		"harvester-csi-driver",
		"harvester-cloud-provider",
		"rke2-ovirt-csi-driver",
	}

	tests := []struct {
		name                  string
		role                  CLIRole
		args                  []string
		wantCloudProvider     string
		wantDisableController bool
		wantDisabled          []string
	}{
		{
			name:         "server without cloud provider disables all provider charts",
			role:         Server,
			wantDisabled: allProviders,
		},
		{
			name:                  "external disables controller and all provider charts",
			role:                  Server,
			args:                  []string{"--cloud-provider-name=external"},
			wantCloudProvider:     "external",
			wantDisableController: true,
			wantDisabled:          allProviders,
		},
		{
			name:                  "vsphere keeps its charts and disables embedded controller",
			role:                  Server,
			args:                  []string{"--cloud-provider-name=rancher-vsphere"},
			wantCloudProvider:     "external",
			wantDisableController: true,
			wantDisabled:          []string{"harvester-csi-driver", "harvester-cloud-provider", "rke2-ovirt-csi-driver"},
		},
		{
			name:                  "harvester keeps its charts and disables embedded controller",
			role:                  Server,
			args:                  []string{"--cloud-provider-name=harvester"},
			wantCloudProvider:     "external",
			wantDisableController: true,
			wantDisabled:          []string{"rancher-vsphere-csi", "rancher-vsphere-cpi", "rke2-ovirt-csi-driver"},
		},
		{
			name:              "CSI-only provider keeps embedded controller enabled",
			role:              Server,
			args:              []string{"--cloud-provider-name=ovirt"},
			wantCloudProvider: "external",
			wantDisabled:      []string{"rancher-vsphere-csi", "rancher-vsphere-cpi", "harvester-csi-driver", "harvester-cloud-provider"},
		},
		{
			name:                  "preserves explicitly disabled charts",
			role:                  Server,
			args:                  []string{"--cloud-provider-name=external", "--disable=rke2-metrics-server"},
			wantCloudProvider:     "external",
			wantDisableController: true,
			wantDisabled:          append([]string{"rke2-metrics-server"}, allProviders...),
		},
		{
			name: "agent without cloud provider does not disable anything",
			role: Agent,
		},
		{
			name:              "external agent does not disable anything",
			role:              Agent,
			args:              []string{"--cloud-provider-name=external"},
			wantCloudProvider: "external",
		},
		{
			name:              "bundled provider agent only rewrites provider name",
			role:              Agent,
			args:              []string{"--cloud-provider-name=rancher-vsphere"},
			wantCloudProvider: "external",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
			flags.String("cloud-provider-name", "", "")
			flags.String("cloud-provider-config", "", "")
			flags.String("node-external-ip", "", "")
			flags.Bool("disable-cloud-controller", false, "")
			flags.Var(cli.NewStringSlice(), "disable", "")
			if err := flags.Parse(tt.args); err != nil {
				t.Fatal(err)
			}
			ctx := cli.NewContext(cli.NewApp(), flags, nil)

			if err := validateCloudProvider(ctx, tt.role); err != nil {
				t.Fatal(err)
			}
			if got := ctx.String("cloud-provider-name"); got != tt.wantCloudProvider {
				t.Errorf("cloud-provider-name = %q, want %q", got, tt.wantCloudProvider)
			}
			if got := ctx.Bool("disable-cloud-controller"); got != tt.wantDisableController {
				t.Errorf("disable-cloud-controller = %v, want %v", got, tt.wantDisableController)
			}
			got := ctx.StringSlice("disable")
			want := tt.wantDisabled
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("disabled charts = %v, want %v", got, want)
			}
		})
	}
}
