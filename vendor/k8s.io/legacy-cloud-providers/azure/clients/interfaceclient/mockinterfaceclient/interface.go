// +build !providerless

/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mockinterfaceclient

import (
	context "context"
	reflect "reflect"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	gomock "github.com/golang/mock/gomock"
	retry "k8s.io/legacy-cloud-providers/azure/retry"
)

// MockInterface is a mock of Interface interface
type MockInterface struct {
	ctrl     *gomock.Controller
	recorder *MockInterfaceMockRecorder
}

// MockInterfaceMockRecorder is the mock recorder for MockInterface
type MockInterfaceMockRecorder struct {
	mock *MockInterface
}

// NewMockInterface creates a new mock instance
func NewMockInterface(ctrl *gomock.Controller) *MockInterface {
	mock := &MockInterface{ctrl: ctrl}
	mock.recorder = &MockInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockInterface) EXPECT() *MockInterfaceMockRecorder {
	return m.recorder
}

// Get mocks base method
func (m *MockInterface) Get(ctx context.Context, resourceGroupName, networkInterfaceName, expand string) (network.Interface, *retry.Error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", ctx, resourceGroupName, networkInterfaceName, expand)
	ret0, _ := ret[0].(network.Interface)
	ret1, _ := ret[1].(*retry.Error)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockInterfaceMockRecorder) Get(ctx, resourceGroupName, networkInterfaceName, expand interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockInterface)(nil).Get), ctx, resourceGroupName, networkInterfaceName, expand)
}

// GetVirtualMachineScaleSetNetworkInterface mocks base method
func (m *MockInterface) GetVirtualMachineScaleSetNetworkInterface(ctx context.Context, resourceGroupName, virtualMachineScaleSetName, virtualmachineIndex, networkInterfaceName, expand string) (network.Interface, *retry.Error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetVirtualMachineScaleSetNetworkInterface", ctx, resourceGroupName, virtualMachineScaleSetName, virtualmachineIndex, networkInterfaceName, expand)
	ret0, _ := ret[0].(network.Interface)
	ret1, _ := ret[1].(*retry.Error)
	return ret0, ret1
}

// GetVirtualMachineScaleSetNetworkInterface indicates an expected call of GetVirtualMachineScaleSetNetworkInterface
func (mr *MockInterfaceMockRecorder) GetVirtualMachineScaleSetNetworkInterface(ctx, resourceGroupName, virtualMachineScaleSetName, virtualmachineIndex, networkInterfaceName, expand interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetVirtualMachineScaleSetNetworkInterface", reflect.TypeOf((*MockInterface)(nil).GetVirtualMachineScaleSetNetworkInterface), ctx, resourceGroupName, virtualMachineScaleSetName, virtualmachineIndex, networkInterfaceName, expand)
}

// CreateOrUpdate mocks base method
func (m *MockInterface) CreateOrUpdate(ctx context.Context, resourceGroupName, networkInterfaceName string, parameters network.Interface) *retry.Error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdate", ctx, resourceGroupName, networkInterfaceName, parameters)
	ret0, _ := ret[0].(*retry.Error)
	return ret0
}

// CreateOrUpdate indicates an expected call of CreateOrUpdate
func (mr *MockInterfaceMockRecorder) CreateOrUpdate(ctx, resourceGroupName, networkInterfaceName, parameters interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdate", reflect.TypeOf((*MockInterface)(nil).CreateOrUpdate), ctx, resourceGroupName, networkInterfaceName, parameters)
}

// Delete mocks base method
func (m *MockInterface) Delete(ctx context.Context, resourceGroupName, networkInterfaceName string) *retry.Error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", ctx, resourceGroupName, networkInterfaceName)
	ret0, _ := ret[0].(*retry.Error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockInterfaceMockRecorder) Delete(ctx, resourceGroupName, networkInterfaceName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockInterface)(nil).Delete), ctx, resourceGroupName, networkInterfaceName)
}
