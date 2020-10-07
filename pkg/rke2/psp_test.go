package rke2

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes"
)

func Test_deployPodSecurityPolicyFromYaml(t *testing.T) {
	type args struct {
		ctx     context.Context
		cs      *kubernetes.Clientset
		pspYaml string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful",
			args: args{
				ctx:     context.Background(),
				cs:      nil,
				pspYaml: "",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := deployPodSecurityPolicyFromYaml(tt.args.ctx, tt.args.cs, tt.args.pspYaml); (err != nil) != tt.wantErr {
				t.Errorf("deployPodSecurityPolicyFromYaml() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
