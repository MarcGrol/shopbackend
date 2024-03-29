// Code generated by MockGen. DO NOT EDIT.
// Source: api.go

// Package myqueue is a generated GoMock package.
package myqueue

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockTaskQueuer is a mock of TaskQueuer interface.
type MockTaskQueuer struct {
	ctrl     *gomock.Controller
	recorder *MockTaskQueuerMockRecorder
}

// MockTaskQueuerMockRecorder is the mock recorder for MockTaskQueuer.
type MockTaskQueuerMockRecorder struct {
	mock *MockTaskQueuer
}

// NewMockTaskQueuer creates a new mock instance.
func NewMockTaskQueuer(ctrl *gomock.Controller) *MockTaskQueuer {
	mock := &MockTaskQueuer{ctrl: ctrl}
	mock.recorder = &MockTaskQueuerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTaskQueuer) EXPECT() *MockTaskQueuerMockRecorder {
	return m.recorder
}

// Enqueue mocks base method.
func (m *MockTaskQueuer) Enqueue(c context.Context, task Task) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Enqueue", c, task)
	ret0, _ := ret[0].(error)
	return ret0
}

// Enqueue indicates an expected call of Enqueue.
func (mr *MockTaskQueuerMockRecorder) Enqueue(c, task interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Enqueue", reflect.TypeOf((*MockTaskQueuer)(nil).Enqueue), c, task)
}

// IsLastAttempt mocks base method.
func (m *MockTaskQueuer) IsLastAttempt(c context.Context, taskUID string) (int32, int32) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsLastAttempt", c, taskUID)
	ret0, _ := ret[0].(int32)
	ret1, _ := ret[1].(int32)
	return ret0, ret1
}

// IsLastAttempt indicates an expected call of IsLastAttempt.
func (mr *MockTaskQueuerMockRecorder) IsLastAttempt(c, taskUID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsLastAttempt", reflect.TypeOf((*MockTaskQueuer)(nil).IsLastAttempt), c, taskUID)
}
