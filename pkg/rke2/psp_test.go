package rke2

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/api/policy/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_deployPodSecurityPolicyFromYaml(t *testing.T) {
	type args struct {
		ctx     context.Context
		cs      kubernetes.Interface
		pspYaml string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successfully create PSP",
			args: args{
				ctx:     context.Background(),
				cs:      fake.NewSimpleClientset(nil),
				pspYaml: fmt.Sprintf(globalRestrictedPSPTemplate, "test-psp"),
			},
			wantErr: false,
		},
		{
			name: "successfully update PSP",
			args: args{
				ctx:     context.Background(),
				cs:      fake.NewSimpleClientset(&v1beta1.PodSecurityPolicy{}),
				pspYaml: fmt.Sprintf(globalRestrictedPSPTemplate, "test-psp"),
			},
			wantErr: false,
		},
		{
			name: "fail to create",
			args: args{
				ctx:     context.Background(),
				cs:      fake.NewSimpleClientset(nil),
				pspYaml: fmt.Sprintf(globalRestrictedPSPTemplate, "test-psp"),
			},
			wantErr: false,
		},
		{
			name: "fail to update",
			args: args{
				ctx:     context.Background(),
				cs:      fake.NewSimpleClientset(nil),
				pspYaml: fmt.Sprintf(globalRestrictedPSPTemplate, "test-psp"),
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
