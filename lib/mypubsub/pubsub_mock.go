// Code generated by MockGen. DO NOT EDIT.
// Source: pubsub_api.go

// Package mypubsub is a generated GoMock package.
package mypubsub

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockPubSub is a mock of PubSub interface.
type MockPubSub struct {
	ctrl     *gomock.Controller
	recorder *MockPubSubMockRecorder
}

// MockPubSubMockRecorder is the mock recorder for MockPubSub.
type MockPubSubMockRecorder struct {
	mock *MockPubSub
}

// NewMockPubSub creates a new mock instance.
func NewMockPubSub(ctrl *gomock.Controller) *MockPubSub {
	mock := &MockPubSub{ctrl: ctrl}
	mock.recorder = &MockPubSubMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPubSub) EXPECT() *MockPubSubMockRecorder {
	return m.recorder
}

// CreateTopic mocks base method.
func (m *MockPubSub) CreateTopic(c context.Context, topic string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTopic", c, topic)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateTopic indicates an expected call of CreateTopic.
func (mr *MockPubSubMockRecorder) CreateTopic(c, topic interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTopic", reflect.TypeOf((*MockPubSub)(nil).CreateTopic), c, topic)
}

// Publish mocks base method.
func (m *MockPubSub) Publish(c context.Context, topic, data string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Publish", c, topic, data)
	ret0, _ := ret[0].(error)
	return ret0
}

// Publish indicates an expected call of Publish.
func (mr *MockPubSubMockRecorder) Publish(c, topic, data interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Publish", reflect.TypeOf((*MockPubSub)(nil).Publish), c, topic, data)
}

// Subscribe mocks base method.
func (m *MockPubSub) Subscribe(c context.Context, topic, urlToPostTo string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subscribe", c, topic, urlToPostTo)
	ret0, _ := ret[0].(error)
	return ret0
}

// Subscribe indicates an expected call of Subscribe.
func (mr *MockPubSubMockRecorder) Subscribe(c, topic, urlToPostTo interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subscribe", reflect.TypeOf((*MockPubSub)(nil).Subscribe), c, topic, urlToPostTo)
}
