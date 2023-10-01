// Code generated by MockGen. DO NOT EDIT.
// Source: api.go
//
// Generated by this command:
//
//	mockgen -source=api.go -package mystore -destination store_mock.go Store
//
// Package mystore is a generated GoMock package.
package mystore

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockStore is a mock of Store interface.
type MockStore[T any] struct {
	ctrl     *gomock.Controller
	recorder *MockStoreMockRecorder[T]
}

// MockStoreMockRecorder is the mock recorder for MockStore.
type MockStoreMockRecorder[T any] struct {
	mock *MockStore[T]
}

// NewMockStore creates a new mock instance.
func NewMockStore[T any](ctrl *gomock.Controller) *MockStore[T] {
	mock := &MockStore[T]{ctrl: ctrl}
	mock.recorder = &MockStoreMockRecorder[T]{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStore[T]) EXPECT() *MockStoreMockRecorder[T] {
	return m.recorder
}

// Get mocks base method.
func (m *MockStore[T]) Get(c context.Context, uid string) (T, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", c, uid)
	ret0, _ := ret[0].(T)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Get indicates an expected call of Get.
func (mr *MockStoreMockRecorder[T]) Get(c, uid any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockStore[T])(nil).Get), c, uid)
}

// List mocks base method.
func (m *MockStore[T]) List(c context.Context) ([]T, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", c)
	ret0, _ := ret[0].([]T)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockStoreMockRecorder[T]) List(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockStore[T])(nil).List), c)
}

// Put mocks base method.
func (m *MockStore[T]) Put(c context.Context, uid string, value T) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Put", c, uid, value)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put.
func (mr *MockStoreMockRecorder[T]) Put(c, uid, value any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockStore[T])(nil).Put), c, uid, value)
}

// Query mocks base method.
func (m *MockStore[T]) Query(c context.Context, filters []Filter, orderByField string) ([]T, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Query", c, filters, orderByField)
	ret0, _ := ret[0].([]T)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Query indicates an expected call of Query.
func (mr *MockStoreMockRecorder[T]) Query(c, filters, orderByField any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Query", reflect.TypeOf((*MockStore[T])(nil).Query), c, filters, orderByField)
}

// RunInTransaction mocks base method.
func (m *MockStore[T]) RunInTransaction(c context.Context, f func(context.Context) error) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunInTransaction", c, f)
	ret0, _ := ret[0].(error)
	return ret0
}

// RunInTransaction indicates an expected call of RunInTransaction.
func (mr *MockStoreMockRecorder[T]) RunInTransaction(c, f any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunInTransaction", reflect.TypeOf((*MockStore[T])(nil).RunInTransaction), c, f)
}
