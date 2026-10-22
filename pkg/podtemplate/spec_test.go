package podtemplate

import (
	"testing"

	rke2cli "github.com/rancher/rke2/pkg/cli"
	v1 "k8s.io/api/core/v1"
)

func Test_UnitAPIServerProbePaths(t *testing.T) {
	tests := []struct {
		name          string
		cfg           rke2cli.Config
		wantLiveness  string
		wantReadiness string
		wantErr       bool
	}{
		{
			name:          "default",
			wantLiveness:  "--raw=/livez",
			wantReadiness: "--raw=/readyz",
		},
		{
			name: "exclude etcd overrides",
			cfg: rke2cli.Config{
				KubeAPIServerLivenessPath:  "/livez?exclude=etcd-override-0",
				KubeAPIServerReadinessPath: "/readyz?exclude=etcd-override-0&exclude=etcd-override-readiness-0",
			},
			wantLiveness:  "--raw=/livez?exclude=etcd-override-0",
			wantReadiness: "--raw=/readyz?exclude=etcd-override-0&exclude=etcd-override-readiness-0",
		},
		{
			name:    "relative path",
			cfg:     rke2cli.Config{KubeAPIServerReadinessPath: "readyz"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewConfigFromCLI(t.TempDir(), tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewConfigFromCLI() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			c.ImagesDir = ""
			spec, err := c.APIServer(nil)
			if err != nil {
				t.Fatalf("APIServer() error = %v", err)
			}
			pod, err := Pod(spec)
			if err != nil {
				t.Fatalf("Pod() error = %v", err)
			}
			ctr := pod.Spec.Containers[0]
			for name, tc := range map[string]struct {
				probe *v1.Probe
				want  string
			}{
				"liveness":  {ctr.LivenessProbe, tt.wantLiveness},
				"startup":   {ctr.StartupProbe, tt.wantLiveness},
				"readiness": {ctr.ReadinessProbe, tt.wantReadiness},
			} {
				cmd := tc.probe.Exec.Command
				if got := cmd[len(cmd)-1]; got != tc.want {
					t.Errorf("%s probe = %q, want %q", name, got, tc.want)
				}
			}
		})
	}
}
