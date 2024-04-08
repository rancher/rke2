//go:build linux
// +build linux

package rke2

import (
	"flag"
	"reflect"
	"testing"

	"github.com/rancher/rke2/pkg/podexecutor"
	"github.com/rancher/rke2/pkg/staticpod"
	"github.com/urfave/cli"
)

func Test_UnitInitExecutor(t *testing.T) {
	type args struct {
		clx      *cli.Context
		cfg      Config
		isServer bool
	}
	tests := []struct {
		name    string
		args    args
		want    *podexecutor.StaticPodConfig
		wantErr bool
	}{
		{
			name: "agent",
			args: args{
				cfg: Config{
					ControlPlaneProbeConf:        []string{"kube-proxy-startup-initial-delay-seconds=42"},
					ControlPlaneResourceLimits:   []string{"kube-proxy-cpu=123m"},
					ControlPlaneResourceRequests: []string{"kube-proxy-memory=123Mi"},
					ExtraEnv:                     ExtraEnv{KubeProxy: []string{"FOO=BAR"}},
					ExtraMounts:                  ExtraMounts{KubeProxy: []string{"/foo=/bar"}},
				},
				isServer: false,
			},
			want: &podexecutor.StaticPodConfig{
				ControlPlaneProbeConfs: podexecutor.ControlPlaneProbeConfs{
					KubeProxy: staticpod.ProbeConfs{
						Startup: staticpod.ProbeConf{
							InitialDelaySeconds: 42,
						},
					},
				},
				ControlPlaneResources: podexecutor.ControlPlaneResources{
					KubeProxyCPULimit:      "123m",
					KubeProxyMemoryRequest: "123Mi",
				},
				ControlPlaneEnv: podexecutor.ControlPlaneEnv{
					KubeProxy: []string{"FOO=BAR"},
				},
				ControlPlaneMounts: podexecutor.ControlPlaneMounts{
					KubeProxy: []string{"/foo=/bar"},
				},
			},
			wantErr: false,
		},
		{
			name: "server",
			args: args{
				cfg: Config{
					ControlPlaneProbeConf:        []string{"kube-proxy-startup-initial-delay-seconds=123"},
					ControlPlaneResourceLimits:   []string{"kube-proxy-cpu=42m"},
					ControlPlaneResourceRequests: []string{"kube-proxy-memory=42Mi"},
					ExtraEnv:                     ExtraEnv{KubeProxy: []string{"BAZ=BOP"}},
					ExtraMounts:                  ExtraMounts{KubeProxy: []string{"/baz=/bop"}},
				},
				isServer: true,
			},
			want: &podexecutor.StaticPodConfig{
				ControlPlaneProbeConfs: podexecutor.ControlPlaneProbeConfs{
					KubeProxy: staticpod.ProbeConfs{
						Startup: staticpod.ProbeConf{
							InitialDelaySeconds: 123,
						},
					},
				},
				ControlPlaneResources: podexecutor.ControlPlaneResources{
					KubeProxyCPULimit:      "42m",
					KubeProxyMemoryRequest: "42Mi",
				},
				ControlPlaneEnv: podexecutor.ControlPlaneEnv{
					KubeProxy: []string{"BAZ=BOP"},
				},
				ControlPlaneMounts: podexecutor.ControlPlaneMounts{
					KubeProxy: []string{"/baz=/bop"},
				},
			},
			wantErr: false,
		},
		{
			name: "bad probe conf",
			args: args{
				cfg: Config{
					ControlPlaneProbeConf: []string{"kube-proxy-startup-initial-delay-seconds=-123"},
				},
			},
			wantErr: true,
		},
		{
			name: "bad control plane limits",
			args: args{
				cfg: Config{
					ControlPlaneResourceLimits: []string{"kube-proxy-cpu"},
				},
			},
			wantErr: true,
		},
		{
			name: "bad control plane requests",
			args: args{
				cfg: Config{
					ControlPlaneResourceRequests: []string{"kube-proxy-memory"},
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
			got, err := initExecutor(tt.args.clx, tt.args.cfg, tt.args.isServer)
			if (err != nil) != tt.wantErr {
				t.Errorf("initExecutor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Don't check the returned struct if we expected an error
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got.ControlPlaneProbeConfs.KubeProxy.Startup.InitialDelaySeconds, tt.want.ControlPlaneProbeConfs.KubeProxy.Startup.InitialDelaySeconds) {
				t.Errorf("initExecutor() kube-proxy-startup-initial-delay-seconds = %+v\nWant = %+v",
					got.ControlPlaneProbeConfs.KubeProxy.Startup.InitialDelaySeconds,
					tt.want.ControlPlaneProbeConfs.KubeProxy.Startup.InitialDelaySeconds)
			}
			if !reflect.DeepEqual(got.ControlPlaneResources.KubeProxyCPULimit, tt.want.ControlPlaneResources.KubeProxyCPULimit) {
				t.Errorf("initExecutor() kube-proxy-cpu = %+v\nWant = %+v",
					got.ControlPlaneResources.KubeProxyCPULimit,
					tt.want.ControlPlaneResources.KubeProxyCPULimit)
			}
			if !reflect.DeepEqual(got.ControlPlaneResources.KubeProxyMemoryRequest, tt.want.ControlPlaneResources.KubeProxyMemoryRequest) {
				t.Errorf("initExecutor() kube-proxy-memory = %+v\nWant = %+v",
					got.ControlPlaneResources.KubeProxyMemoryRequest,
					tt.want.ControlPlaneResources.KubeProxyMemoryRequest)
			}
			if !reflect.DeepEqual(got.ControlPlaneEnv.KubeProxy, tt.want.ControlPlaneEnv.KubeProxy) {
				t.Errorf("initExecutor() kube-proxy extra-env = %+v\nWant = %+v",
					got.ControlPlaneEnv.KubeProxy,
					tt.want.ControlPlaneEnv.KubeProxy)
			}
			if !reflect.DeepEqual(got.ControlPlaneMounts.KubeProxy, tt.want.ControlPlaneMounts.KubeProxy) {
				t.Errorf("initExecutor() kube-proxy extra-mounts = %+v\nWant = %+v",
					got.ControlPlaneMounts.KubeProxy,
					tt.want.ControlPlaneMounts.KubeProxy)
			}
		})
	}
}
