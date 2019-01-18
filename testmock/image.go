// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/buildpack/lifecycle/image (interfaces: Image)

// Package testmock is a generated GoMock package.
package testmock

import (
	image "github.com/buildpack/lifecycle/image"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockImage is a mock of Image interface
type MockImage struct {
	ctrl     *gomock.Controller
	recorder *MockImageMockRecorder
}

// MockImageMockRecorder is the mock recorder for MockImage
type MockImageMockRecorder struct {
	mock *MockImage
}

// NewMockImage creates a new mock instance
func NewMockImage(ctrl *gomock.Controller) *MockImage {
	mock := &MockImage{ctrl: ctrl}
	mock.recorder = &MockImageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockImage) EXPECT() *MockImageMockRecorder {
	return m.recorder
}

// AddLayer mocks base method
func (m *MockImage) AddLayer(arg0 string) error {
	ret := m.ctrl.Call(m, "AddLayer", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// AddLayer indicates an expected call of AddLayer
func (mr *MockImageMockRecorder) AddLayer(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddLayer", reflect.TypeOf((*MockImage)(nil).AddLayer), arg0)
}

// Digest mocks base method
func (m *MockImage) Digest() (string, error) {
	ret := m.ctrl.Call(m, "Digest")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Digest indicates an expected call of Digest
func (mr *MockImageMockRecorder) Digest() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Digest", reflect.TypeOf((*MockImage)(nil).Digest))
}

// Env mocks base method
func (m *MockImage) Env(arg0 string) (string, error) {
	ret := m.ctrl.Call(m, "Env", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Env indicates an expected call of Env
func (mr *MockImageMockRecorder) Env(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Env", reflect.TypeOf((*MockImage)(nil).Env), arg0)
}

// Label mocks base method
func (m *MockImage) Label(arg0 string) (string, error) {
	ret := m.ctrl.Call(m, "Label", arg0)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Label indicates an expected call of Label
func (mr *MockImageMockRecorder) Label(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Label", reflect.TypeOf((*MockImage)(nil).Label), arg0)
}

// Name mocks base method
func (m *MockImage) Name() string {
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name
func (mr *MockImageMockRecorder) Name() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockImage)(nil).Name))
}

// Rebase mocks base method
func (m *MockImage) Rebase(arg0 string, arg1 image.Image) error {
	ret := m.ctrl.Call(m, "Rebase", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Rebase indicates an expected call of Rebase
func (mr *MockImageMockRecorder) Rebase(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Rebase", reflect.TypeOf((*MockImage)(nil).Rebase), arg0, arg1)
}

// Rename mocks base method
func (m *MockImage) Rename(arg0 string) {
	m.ctrl.Call(m, "Rename", arg0)
}

// Rename indicates an expected call of Rename
func (mr *MockImageMockRecorder) Rename(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Rename", reflect.TypeOf((*MockImage)(nil).Rename), arg0)
}

// ReuseLayer mocks base method
func (m *MockImage) ReuseLayer(arg0 string) error {
	ret := m.ctrl.Call(m, "ReuseLayer", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// ReuseLayer indicates an expected call of ReuseLayer
func (mr *MockImageMockRecorder) ReuseLayer(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReuseLayer", reflect.TypeOf((*MockImage)(nil).ReuseLayer), arg0)
}

// Save mocks base method
func (m *MockImage) Save() (string, error) {
	ret := m.ctrl.Call(m, "Save")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Save indicates an expected call of Save
func (mr *MockImageMockRecorder) Save() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Save", reflect.TypeOf((*MockImage)(nil).Save))
}

// SetCmd mocks base method
func (m *MockImage) SetCmd(arg0 ...string) error {
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "SetCmd", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetCmd indicates an expected call of SetCmd
func (mr *MockImageMockRecorder) SetCmd(arg0 ...interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetCmd", reflect.TypeOf((*MockImage)(nil).SetCmd), arg0...)
}

// SetEntrypoint mocks base method
func (m *MockImage) SetEntrypoint(arg0 ...string) error {
	varargs := []interface{}{}
	for _, a := range arg0 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "SetEntrypoint", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetEntrypoint indicates an expected call of SetEntrypoint
func (mr *MockImageMockRecorder) SetEntrypoint(arg0 ...interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetEntrypoint", reflect.TypeOf((*MockImage)(nil).SetEntrypoint), arg0...)
}

// SetEnv mocks base method
func (m *MockImage) SetEnv(arg0, arg1 string) error {
	ret := m.ctrl.Call(m, "SetEnv", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetEnv indicates an expected call of SetEnv
func (mr *MockImageMockRecorder) SetEnv(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetEnv", reflect.TypeOf((*MockImage)(nil).SetEnv), arg0, arg1)
}

// SetLabel mocks base method
func (m *MockImage) SetLabel(arg0, arg1 string) error {
	ret := m.ctrl.Call(m, "SetLabel", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetLabel indicates an expected call of SetLabel
func (mr *MockImageMockRecorder) SetLabel(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetLabel", reflect.TypeOf((*MockImage)(nil).SetLabel), arg0, arg1)
}

// TopLayer mocks base method
func (m *MockImage) TopLayer() (string, error) {
	ret := m.ctrl.Call(m, "TopLayer")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// TopLayer indicates an expected call of TopLayer
func (mr *MockImageMockRecorder) TopLayer() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TopLayer", reflect.TypeOf((*MockImage)(nil).TopLayer))
}
