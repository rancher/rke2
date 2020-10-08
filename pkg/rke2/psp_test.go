package rke2

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

var testPodSecurityPolicy = &v1beta1.PodSecurityPolicy{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-psp",
	},
}

var testClusterRole = &rbacv1.ClusterRole{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-cluster-role",
	},
}

var testClusterRoleBinding = &rbacv1.ClusterRoleBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-cluster-role-binding",
	},
}

var testRoleBinding = &rbacv1.RoleBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-role-binding",
	},
}

func Test_deployPodSecurityPolicyFromYaml(t *testing.T) {
	pspYAML := fmt.Sprintf(globalRestrictedPSPTemplate, "test-psp")
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
				cs:      fake.NewSimpleClientset(&v1beta1.PodSecurityPolicy{}),
				pspYaml: pspYAML,
			},
			wantErr: false,
		},
		{
			name: "fail to decode YAML",
			args: args{
				ctx:     context.Background(),
				cs:      fake.NewSimpleClientset(testPodSecurityPolicy),
				pspYaml: "",
			},
			wantErr: true,
		},
		{
			name: "successfully update PSP",
			args: args{
				ctx:     context.Background(),
				cs:      fake.NewSimpleClientset(testPodSecurityPolicy),
				pspYaml: pspYAML,
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

func Test_deployClusterRoleBindingFromYaml(t *testing.T) {
	type args struct {
		ctx                    context.Context
		cs                     kubernetes.Interface
		clusterRoleBindingYaml string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{

		{
			name: "successfully create cluster role binding",
			args: args{
				ctx:                    context.Background(),
				cs:                     fake.NewSimpleClientset(&rbacv1.ClusterRoleBinding{}),
				clusterRoleBindingYaml: fmt.Sprintf(kubeletAPIServerRoleBindingTemplate, "test-cluster-role-binding"),
			},
			wantErr: false,
		},
		{
			name: "fail to decode YAML",
			args: args{
				ctx:                    context.Background(),
				cs:                     fake.NewSimpleClientset(testClusterRoleBinding),
				clusterRoleBindingYaml: "",
			},
			wantErr: true,
		},
		{
			name: "successfully update cluster role binding",
			args: args{
				ctx:                    context.Background(),
				cs:                     fake.NewSimpleClientset(testClusterRoleBinding),
				clusterRoleBindingYaml: fmt.Sprintf(kubeletAPIServerRoleBindingTemplate, "test-cluster-role-binding"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := deployClusterRoleBindingFromYaml(tt.args.ctx, tt.args.cs, tt.args.clusterRoleBindingYaml); (err != nil) != tt.wantErr {
				t.Errorf("deployClusterRoleBindingFromYaml() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_deployClusterRoleFromYaml(t *testing.T) {
	type args struct {
		ctx             context.Context
		cs              kubernetes.Interface
		clusterRoleYaml string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successfully create cluster role",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(&rbacv1.ClusterRole{}),
				clusterRoleYaml: fmt.Sprintf(roleTemplate, "test-cluster-role", "test-resource-name"),
			},
			wantErr: false,
		},
		{
			name: "fail to decode YAML",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(testClusterRole),
				clusterRoleYaml: "",
			},
			wantErr: true,
		},
		{
			name: "successfully update cluster role",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(testClusterRole),
				clusterRoleYaml: fmt.Sprintf(roleTemplate, "test-cluster-role", "test-resource-name"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := deployClusterRoleFromYaml(tt.args.ctx, tt.args.cs, tt.args.clusterRoleYaml); (err != nil) != tt.wantErr {
				t.Errorf("deployClusterRoleFromYaml() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_deployRoleBindingFromYaml(t *testing.T) {
	type args struct {
		ctx             context.Context
		cs              kubernetes.Interface
		roleBindingYaml string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successfully create role binding",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(&rbacv1.RoleBinding{}),
				roleBindingYaml: fmt.Sprintf(tunnelControllerRoleTemplate, "test-role-binding"),
			},
			wantErr: false,
		},
		{
			name: "fail to decode YAML",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(testRoleBinding),
				roleBindingYaml: "",
			},
			wantErr: true,
		},
		{
			name: "successfully update role binding",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(testRoleBinding),
				roleBindingYaml: fmt.Sprintf(tunnelControllerRoleTemplate, "test-role-binding"),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := deployRoleBindingFromYaml(tt.args.ctx, tt.args.cs, tt.args.roleBindingYaml); (err != nil) != tt.wantErr {
				t.Errorf("deployRoleBindingFromYaml() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
