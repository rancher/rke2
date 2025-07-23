//go:build linux
// +build linux

package rke2

import (
	"flag"
	"reflect"
	"testing"

	rke2cli "github.com/rancher/rke2/pkg/cli"
	"github.com/rancher/rke2/pkg/executor/staticpod"
	"github.com/rancher/rke2/pkg/podtemplate"
	"github.com/urfave/cli/v2"
)

func Test_UnitInitExecutor(t *testing.T) {
	type args struct {
		clx      *cli.Context
		cfg      rke2cli.Config
		isServer bool
	}
	tests := []struct {
		name    string
		args    args
		want    *staticpod.StaticPodConfig
		wantErr bool
	}{
		{
			name: "agent",
			args: args{
				cfg: rke2cli.Config{
					ControlPlaneProbeConf:        *cli.NewStringSlice("kube-proxy-startup-initial-delay-seconds=42"),
					ControlPlaneResourceLimits:   *cli.NewStringSlice("kube-proxy-cpu=123m"),
					ControlPlaneResourceRequests: *cli.NewStringSlice("kube-proxy-memory=123Mi"),
					ExtraEnv:                     rke2cli.ExtraEnv{KubeProxy: *cli.NewStringSlice("FOO=BAR")},
					ExtraMounts:                  rke2cli.ExtraMounts{KubeProxy: *cli.NewStringSlice("/foo=/bar")},
				},
				isServer: false,
			},
			want: &staticpod.StaticPodConfig{
				Config: podtemplate.Config{
					Probes: &podtemplate.ControlPlaneProbeConfs{
						KubeProxy: podtemplate.ProbeConfs{
							Startup: podtemplate.ProbeConf{
								InitialDelaySeconds: 42,
							},
						},
					},
					Resources: &podtemplate.ControlPlaneResources{
						KubeProxyCPULimit:      "123m",
						KubeProxyMemoryRequest: "123Mi",
					},
					Env: &podtemplate.ControlPlaneEnv{
						KubeProxy: []string{"FOO=BAR"},
					},
					Mounts: &podtemplate.ControlPlaneMounts{
						KubeProxy: []string{"/foo=/bar"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "server",
			args: args{
				cfg: rke2cli.Config{
					ControlPlaneProbeConf:        *cli.NewStringSlice("kube-proxy-startup-initial-delay-seconds=123"),
					ControlPlaneResourceLimits:   *cli.NewStringSlice("kube-proxy-cpu=42m"),
					ControlPlaneResourceRequests: *cli.NewStringSlice("kube-proxy-memory=42Mi"),
					ExtraEnv:                     rke2cli.ExtraEnv{KubeProxy: *cli.NewStringSlice("BAZ=BOP")},
					ExtraMounts:                  rke2cli.ExtraMounts{KubeProxy: *cli.NewStringSlice("/baz=/bop")},
				},
				isServer: true,
			},
			want: &staticpod.StaticPodConfig{
				Config: podtemplate.Config{
					Probes: &podtemplate.ControlPlaneProbeConfs{
						KubeProxy: podtemplate.ProbeConfs{
							Startup: podtemplate.ProbeConf{
								InitialDelaySeconds: 123,
							},
						},
					},
					Resources: &podtemplate.ControlPlaneResources{
						KubeProxyCPULimit:      "42m",
						KubeProxyMemoryRequest: "42Mi",
					},
					Env: &podtemplate.ControlPlaneEnv{
						KubeProxy: []string{"BAZ=BOP"},
					},
					Mounts: &podtemplate.ControlPlaneMounts{
						KubeProxy: []string{"/baz=/bop"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "bad probe conf",
			args: args{
				cfg: rke2cli.Config{
					ControlPlaneProbeConf: *cli.NewStringSlice("kube-proxy-startup-initial-delay-seconds=-123"),
				},
			},
			wantErr: true,
		},
		{
			name: "bad control plane limits",
			args: args{
				cfg: rke2cli.Config{
					ControlPlaneResourceLimits: *cli.NewStringSlice("kube-proxy-cpu"),
				},
			},
			wantErr: true,
		},
		{
			name: "bad control plane requests",
			args: args{
				cfg: rke2cli.Config{
					ControlPlaneResourceRequests: *cli.NewStringSlice("kube-proxy-memory"),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Override the pss location so we attempt to create a file that needs sudo, not what we are testing anyways
			flagSet := flag.NewFlagSet("test", 0)
			flagSet.String("pod-security-admission-config-file", "/tmp/pss.yaml", "")
			tt.args.clx = cli.NewContext(nil, flagSet, nil)
			execer, err := initExecutor(tt.args.clx, tt.args.cfg, tt.args.isServer)
			if (err != nil) != tt.wantErr {
				t.Errorf("initExecutor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Don't check the returned struct if we expected an error
			if tt.wantErr {
				return
			}
			got, ok := execer.(*staticpod.StaticPodConfig)
			if !ok {
				t.Errorf("failed to convert Executor as StaticPodConfig")
				return
			}

			if !reflect.DeepEqual(got.Config.Probes.KubeProxy.Startup.InitialDelaySeconds, tt.want.Config.Probes.KubeProxy.Startup.InitialDelaySeconds) {
				t.Errorf("initExecutor() kube-proxy-startup-initial-delay-seconds = %+v\nWant = %+v",
					got.Config.Probes.KubeProxy.Startup.InitialDelaySeconds,
					tt.want.Config.Probes.KubeProxy.Startup.InitialDelaySeconds)
			}
			if !reflect.DeepEqual(got.Config.Resources.KubeProxyCPULimit, tt.want.Config.Resources.KubeProxyCPULimit) {
				t.Errorf("initExecutor() kube-proxy-cpu = %+v\nWant = %+v",
					got.Config.Resources.KubeProxyCPULimit,
					tt.want.Config.Resources.KubeProxyCPULimit)
			}
			if !reflect.DeepEqual(got.Config.Resources.KubeProxyMemoryRequest, tt.want.Config.Resources.KubeProxyMemoryRequest) {
				t.Errorf("initExecutor() kube-proxy-memory = %+v\nWant = %+v",
					got.Config.Resources.KubeProxyMemoryRequest,
					tt.want.Config.Resources.KubeProxyMemoryRequest)
			}
			if !reflect.DeepEqual(got.Config.Env.KubeProxy, tt.want.Config.Env.KubeProxy) {
				t.Errorf("initExecutor() kube-proxy extra-env = %+v\nWant = %+v",
					got.Config.Env.KubeProxy,
					tt.want.Config.Env.KubeProxy)
			}
			if !reflect.DeepEqual(got.Config.Mounts.KubeProxy, tt.want.Config.Mounts.KubeProxy) {
				t.Errorf("initExecutor() kube-proxy extra-mounts = %+v\nWant = %+v",
					got.Config.Mounts.KubeProxy,
					tt.want.Config.Mounts.KubeProxy)
			}
		})
	}
}
