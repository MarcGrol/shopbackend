// Code generated by MockGen. DO NOT EDIT.
// Source: challenge.go

// Package challenge is a generated GoMock package.
package challenge

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockRandomStringer is a mock of RandomStringer interface.
type MockRandomStringer struct {
	ctrl     *gomock.Controller
	recorder *MockRandomStringerMockRecorder
}

// MockRandomStringerMockRecorder is the mock recorder for MockRandomStringer.
type MockRandomStringerMockRecorder struct {
	mock *MockRandomStringer
}

// NewMockRandomStringer creates a new mock instance.
func NewMockRandomStringer(ctrl *gomock.Controller) *MockRandomStringer {
	mock := &MockRandomStringer{ctrl: ctrl}
	mock.recorder = &MockRandomStringerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRandomStringer) EXPECT() *MockRandomStringerMockRecorder {
	return m.recorder
}

// Create mocks base method.
func (m *MockRandomStringer) Create() (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockRandomStringerMockRecorder) Create() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockRandomStringer)(nil).Create))
}