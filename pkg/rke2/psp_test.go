package rke2

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	fakepolicyv1beta1 "k8s.io/client-go/kubernetes/typed/policy/v1beta1/fake"
	fakerbacv1 "k8s.io/client-go/kubernetes/typed/rbac/v1/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	testPSPName                = "test-psp"
	testClusterRoleName        = "test-cluster-role"
	testClusterRoleBindingName = "test-cluster-role-binding"
	testRoleBindingName        = "test-role-binding"
)

var testPodSecurityPolicy = &v1beta1.PodSecurityPolicy{
	ObjectMeta: metav1.ObjectMeta{
		Name: testPSPName,
	},
}

var testClusterRole = &rbacv1.ClusterRole{
	ObjectMeta: metav1.ObjectMeta{
		Name: testClusterRoleName,
	},
}

var testClusterRoleBinding = &rbacv1.ClusterRoleBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name: testClusterRoleBindingName,
	},
}

var testRoleBinding = &rbacv1.RoleBinding{
	ObjectMeta: metav1.ObjectMeta{
		Name: testRoleBindingName,
	},
}

// fakeWithNonretriableError receives a value of type runtime.Object,
// determines underlying underlying type, and creates a new value of
// type fake.Clientset pointer and sets a Reactor to return an error
// that is not retriable.
func fakeWithNonretriableError(ro interface{}) *fake.Clientset {
	cs := fake.NewSimpleClientset(testPodSecurityPolicy)
	const errMsg = "non retriable error"
	switch ro.(type) {
	case *v1beta1.PodSecurityPolicy:
		cs.PolicyV1beta1().(*fakepolicyv1beta1.FakePolicyV1beta1).PrependReactor("update", "*",
			func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &v1beta1.PodSecurityPolicy{}, errors.New(errMsg)
			},
		)
	case *rbacv1.ClusterRoleBinding:
		cs.RbacV1().(*fakerbacv1.FakeRbacV1).PrependReactor("*", "*",
			func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &rbacv1.ClusterRoleBinding{}, errors.New(errMsg)
			},
		)
	case *rbacv1.ClusterRole:
		cs.RbacV1().(*fakerbacv1.FakeRbacV1).PrependReactor("*", "*",
			func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &rbacv1.ClusterRole{}, errors.New(errMsg)
			},
		)
	case *rbacv1.RoleBinding:
		cs.RbacV1().(*fakerbacv1.FakeRbacV1).PrependReactor("*", "*",
			func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &rbacv1.RoleBinding{}, errors.New(errMsg)
			},
		)
	}
	return cs
}

// fakeWithRetriableError creates a new value of fake.Clientset
// pointer and sets a Reactor to return an error that will be
// caught by retry logic.
func fakeWithRetriableError(ro interface{}) *fake.Clientset {
	cs := fake.NewSimpleClientset(testPodSecurityPolicy)
	switch ro.(type) {
	case *v1beta1.PodSecurityPolicy:
		cs.PolicyV1beta1().(*fakepolicyv1beta1.FakePolicyV1beta1).PrependReactor("update", "*",
			func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &v1beta1.PodSecurityPolicy{},
					k8serrors.NewConflict(schema.GroupResource{
						Resource: "psp",
					},
						"psp-update", nil,
					)
			},
		)
	case *rbacv1.ClusterRoleBinding:
		cs.RbacV1().(*fakerbacv1.FakeRbacV1).PrependReactor("*", "*",
			func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &rbacv1.ClusterRoleBinding{},
					k8serrors.NewConflict(schema.GroupResource{
						Resource: "clusterolebindings",
					},
						"cluster-role-binding-update", nil,
					)
			},
		)
	case *rbacv1.ClusterRole:
		cs.RbacV1().(*fakerbacv1.FakeRbacV1).PrependReactor("*", "*",
			func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &rbacv1.ClusterRole{},
					k8serrors.NewConflict(schema.GroupResource{
						Resource: "clusterrole",
					},
						"cluster-role-update", nil,
					)
			},
		)
	case *rbacv1.RoleBinding:
		cs.RbacV1().(*fakerbacv1.FakeRbacV1).PrependReactor("*", "*",
			func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &rbacv1.RoleBinding{},
					k8serrors.NewConflict(schema.GroupResource{
						Resource: "rolebindings",
					},
						"role-binding-update", nil,
					)
			},
		)
	}
	return cs
}

func Test_deployPodSecurityPolicyFromYaml(t *testing.T) {
	pspYAML := fmt.Sprintf(globalRestrictedPSPTemplate, testPSPName)
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
			name: "fail to decode YAML",
			args: args{
				ctx:     context.TODO(),
				cs:      fake.NewSimpleClientset(testPodSecurityPolicy),
				pspYaml: "",
			},
			wantErr: true,
		},
		{
			name: "successfully create PSP",
			args: args{
				ctx:     context.TODO(),
				cs:      fake.NewSimpleClientset(&v1beta1.PodSecurityPolicy{}),
				pspYaml: pspYAML,
			},
			wantErr: false,
		},
		{
			name: "successfully update PSP",
			args: args{
				ctx:     context.TODO(),
				cs:      fake.NewSimpleClientset(testPodSecurityPolicy),
				pspYaml: pspYAML,
			},
			wantErr: false,
		},
		{
			name: "fail update PSP - nonretriable",
			args: args{
				ctx:     context.TODO(),
				cs:      fakeWithNonretriableError(&v1beta1.PodSecurityPolicy{}),
				pspYaml: pspYAML,
			},
			wantErr: true,
		},
		{
			name: "fail update PSP - retriable error",
			args: args{
				ctx:     context.TODO(),
				cs:      fakeWithRetriableError(&v1beta1.PodSecurityPolicy{}),
				pspYaml: pspYAML,
			},
			wantErr: true,
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
	clusterRoleBindingYaml := fmt.Sprintf(kubeletAPIServerRoleBindingTemplate, testClusterRoleBindingName)
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
			name: "fail to decode YAML",
			args: args{
				ctx:                    context.Background(),
				cs:                     fake.NewSimpleClientset(testClusterRoleBinding),
				clusterRoleBindingYaml: "",
			},
			wantErr: true,
		},
		{
			name: "successfully create cluster role binding",
			args: args{
				ctx:                    context.Background(),
				cs:                     fake.NewSimpleClientset(&rbacv1.ClusterRoleBinding{}),
				clusterRoleBindingYaml: clusterRoleBindingYaml,
			},
			wantErr: false,
		},
		{
			name: "successfully update cluster role binding",
			args: args{
				ctx:                    context.Background(),
				cs:                     fake.NewSimpleClientset(testClusterRoleBinding),
				clusterRoleBindingYaml: clusterRoleBindingYaml,
			},
			wantErr: false,
		},
		{
			name: "fail update cluster role binding - nonretriable",
			args: args{
				ctx:                    context.TODO(),
				cs:                     fakeWithNonretriableError(&rbacv1.ClusterRoleBinding{}),
				clusterRoleBindingYaml: clusterRoleBindingYaml,
			},
			wantErr: true,
		},
		{
			name: "fail update cluster role binding - retriable error",
			args: args{
				ctx:                    context.TODO(),
				cs:                     fakeWithRetriableError(&rbacv1.ClusterRoleBinding{}),
				clusterRoleBindingYaml: clusterRoleBindingYaml,
			},
			wantErr: true,
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
	const testResourceName = "test-resource-name"
	clusterRoleYaml := fmt.Sprintf(roleTemplate, "test-cluster-role", testResourceName)
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
			name: "fail to decode YAML",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(testClusterRole),
				clusterRoleYaml: "",
			},
			wantErr: true,
		},
		{
			name: "successfully create cluster role",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(&rbacv1.ClusterRole{}),
				clusterRoleYaml: clusterRoleYaml,
			},
			wantErr: false,
		},
		{
			name: "successfully update cluster role",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(testClusterRole),
				clusterRoleYaml: clusterRoleYaml,
			},
			wantErr: false,
		},
		{
			name: "fail update cluster role binding - nonretriable",
			args: args{
				ctx:             context.TODO(),
				cs:              fakeWithNonretriableError(&rbacv1.ClusterRole{}),
				clusterRoleYaml: clusterRoleYaml,
			},
			wantErr: true,
		},
		{
			name: "fail update cluster role binding - retriable error",
			args: args{
				ctx:             context.TODO(),
				cs:              fakeWithRetriableError(&rbacv1.ClusterRole{}),
				clusterRoleYaml: clusterRoleYaml,
			},
			wantErr: true,
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
	roleBindingYaml := fmt.Sprintf(tunnelControllerRoleTemplate, testRoleBindingName)
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
			name: "fail to decode YAML",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(testRoleBinding),
				roleBindingYaml: "",
			},
			wantErr: true,
		},
		{
			name: "successfully create role binding",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(&rbacv1.RoleBinding{}),
				roleBindingYaml: roleBindingYaml,
			},
			wantErr: false,
		},
		{
			name: "successfully update role binding",
			args: args{
				ctx:             context.Background(),
				cs:              fake.NewSimpleClientset(testRoleBinding),
				roleBindingYaml: roleBindingYaml,
			},
			wantErr: false,
		},
		{
			name: "fail update role binding - nonretriable",
			args: args{
				ctx:             context.TODO(),
				cs:              fakeWithNonretriableError(&rbacv1.RoleBinding{}),
				roleBindingYaml: roleBindingYaml,
			},
			wantErr: true,
		},
		{
			name: "fail update role binding - retriable error",
			args: args{
				ctx:             context.TODO(),
				cs:              fakeWithRetriableError(&rbacv1.RoleBinding{}),
				roleBindingYaml: roleBindingYaml,
			},
			wantErr: true,
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
