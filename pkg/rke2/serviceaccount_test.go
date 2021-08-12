package rke2

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
)

var testServiceAccountEmpty = &v1.ServiceAccount{
	ObjectMeta: metav1.ObjectMeta{
		Name: "default",
	},
}
var testServiceAccountFilled = &v1.ServiceAccount{
	ObjectMeta: metav1.ObjectMeta{
		Name: "default",
	},
	AutomountServiceAccountToken: pointer.BoolPtr(false),
}

func addClientReactors(cs *fake.Clientset, verb string, pass bool) *fake.Clientset {
	switch verb {
	case "get":
		if pass {
			cs.AddReactor(verb, "serviceaccounts",
				func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, testServiceAccountEmpty, nil
				},
			)
		} else {
			cs.AddReactor(verb, "serviceaccounts",
				func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1.ServiceAccount{},
						k8serrors.NewNotFound(
							schema.GroupResource{Resource: "sa"},
							"sa-get",
						)
				},
			)
		}
	case "update":
		if pass {
			cs.AddReactor(verb, "serviceaccounts",
				func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, testServiceAccountFilled, nil
				},
			)
		} else {
			cs.AddReactor(verb, "serviceaccounts",
				func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, &v1.ServiceAccount{},
						k8serrors.NewConflict(
							schema.GroupResource{Resource: "sa"},
							"sa-update",
							nil,
						)
				},
			)
		}
	}
	return cs
}

func Test_UnitrestrictServiceAccount(t *testing.T) {
	type args struct {
		ctx       context.Context
		namespace string
		cs        *fake.Clientset
	}
	tests := []struct {
		name    string
		args    args
		setup   func(*fake.Clientset)
		wantErr bool
	}{
		{
			name: "Succeed on get and update",
			args: args{
				ctx:       context.Background(),
				namespace: "default",
				cs:        &fake.Clientset{},
			},
			setup: func(cs *fake.Clientset) {
				addClientReactors(cs, "get", true)
				addClientReactors(cs, "update", true)
			},
			wantErr: false,
		},
		{
			name: "Fail on get",
			args: args{
				ctx:       context.Background(),
				namespace: "default",
				cs:        &fake.Clientset{},
			},
			setup: func(cs *fake.Clientset) {
				addClientReactors(cs, "get", false)
				addClientReactors(cs, "update", false)
			},
			wantErr: true,
		},
		{
			name: "Fail on update",
			args: args{
				ctx:       context.Background(),
				namespace: "default",
				cs:        &fake.Clientset{},
			},
			setup: func(cs *fake.Clientset) {
				addClientReactors(cs, "get", true)
				addClientReactors(cs, "update", false)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(tt.args.cs)
			if err := restrictServiceAccount(tt.args.ctx, tt.args.namespace, tt.args.cs); (err != nil) != tt.wantErr {
				t.Errorf("restrictServiceAccount() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
