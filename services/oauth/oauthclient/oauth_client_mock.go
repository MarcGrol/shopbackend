// Code generated by MockGen. DO NOT EDIT.
// Source: oauth_client.go

// Package oauthclient is a generated GoMock package.
package oauthclient

import (
	context "context"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockOauthClient is a mock of OauthClient interface.
type MockOauthClient struct {
	ctrl     *gomock.Controller
	recorder *MockOauthClientMockRecorder
}

// MockOauthClientMockRecorder is the mock recorder for MockOauthClient.
type MockOauthClientMockRecorder struct {
	mock *MockOauthClient
}

// NewMockOauthClient creates a new mock instance.
func NewMockOauthClient(ctrl *gomock.Controller) *MockOauthClient {
	mock := &MockOauthClient{ctrl: ctrl}
	mock.recorder = &MockOauthClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockOauthClient) EXPECT() *MockOauthClientMockRecorder {
	return m.recorder
}

// CancelAccessToken mocks base method.
func (m *MockOauthClient) CancelAccessToken(c context.Context, req CancelTokenRequest) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CancelAccessToken", c, req)
	ret0, _ := ret[0].(error)
	return ret0
}

// CancelAccessToken indicates an expected call of CancelAccessToken.
func (mr *MockOauthClientMockRecorder) CancelAccessToken(c, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CancelAccessToken", reflect.TypeOf((*MockOauthClient)(nil).CancelAccessToken), c, req)
}

// ComposeAuthURL mocks base method.
func (m *MockOauthClient) ComposeAuthURL(c context.Context, req ComposeAuthURLRequest) (string, string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ComposeAuthURL", c, req)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ComposeAuthURL indicates an expected call of ComposeAuthURL.
func (mr *MockOauthClientMockRecorder) ComposeAuthURL(c, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ComposeAuthURL", reflect.TypeOf((*MockOauthClient)(nil).ComposeAuthURL), c, req)
}

// GetAccessToken mocks base method.
func (m *MockOauthClient) GetAccessToken(c context.Context, req GetTokenRequest) (GetTokenResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAccessToken", c, req)
	ret0, _ := ret[0].(GetTokenResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAccessToken indicates an expected call of GetAccessToken.
func (mr *MockOauthClientMockRecorder) GetAccessToken(c, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAccessToken", reflect.TypeOf((*MockOauthClient)(nil).GetAccessToken), c, req)
}

// RefreshAccessToken mocks base method.
func (m *MockOauthClient) RefreshAccessToken(c context.Context, req RefreshTokenRequest) (GetTokenResponse, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RefreshAccessToken", c, req)
	ret0, _ := ret[0].(GetTokenResponse)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RefreshAccessToken indicates an expected call of RefreshAccessToken.
func (mr *MockOauthClientMockRecorder) RefreshAccessToken(c, req interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RefreshAccessToken", reflect.TypeOf((*MockOauthClient)(nil).RefreshAccessToken), c, req)
}
