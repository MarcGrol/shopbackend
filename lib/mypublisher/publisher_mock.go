// Code generated by MockGen. DO NOT EDIT.
// Source: api.go

// Package mypublisher is a generated GoMock package.
package mypublisher

import (
	context "context"
	reflect "reflect"

	myevents "github.com/MarcGrol/shopbackend/lib/myevents"
	gomock "go.uber.org/mock/gomock"
)

// MockPublisher is a mock of Publisher interface.
type MockPublisher struct {
	ctrl     *gomock.Controller
	recorder *MockPublisherMockRecorder
}

// MockPublisherMockRecorder is the mock recorder for MockPublisher.
type MockPublisherMockRecorder struct {
	mock *MockPublisher
}

// NewMockPublisher creates a new mock instance.
func NewMockPublisher(ctrl *gomock.Controller) *MockPublisher {
	mock := &MockPublisher{ctrl: ctrl}
	mock.recorder = &MockPublisherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPublisher) EXPECT() *MockPublisherMockRecorder {
	return m.recorder
}

// CreateTopic mocks base method.
func (m *MockPublisher) CreateTopic(ctx context.Context, topicName string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateTopic", ctx, topicName)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateTopic indicates an expected call of CreateTopic.
func (mr *MockPublisherMockRecorder) CreateTopic(ctx, topicName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateTopic", reflect.TypeOf((*MockPublisher)(nil).CreateTopic), ctx, topicName)
}

// Publish mocks base method.
func (m *MockPublisher) Publish(c context.Context, topic string, env myevents.Event) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Publish", c, topic, env)
	ret0, _ := ret[0].(error)
	return ret0
}

// Publish indicates an expected call of Publish.
func (mr *MockPublisherMockRecorder) Publish(c, topic, env interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Publish", reflect.TypeOf((*MockPublisher)(nil).Publish), c, topic, env)
}
