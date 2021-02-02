// Code generated by MockGen. DO NOT EDIT.
// Source: encode.go

// Package mock_event is a generated GoMock package.
package mock_event

import (
	gomock "github.com/golang/mock/gomock"
	event "github.com/modernice/goes/event"
	io "io"
	reflect "reflect"
)

// MockEncoder is a mock of Encoder interface
type MockEncoder struct {
	ctrl     *gomock.Controller
	recorder *MockEncoderMockRecorder
}

// MockEncoderMockRecorder is the mock recorder for MockEncoder
type MockEncoderMockRecorder struct {
	mock *MockEncoder
}

// NewMockEncoder creates a new mock instance
func NewMockEncoder(ctrl *gomock.Controller) *MockEncoder {
	mock := &MockEncoder{ctrl: ctrl}
	mock.recorder = &MockEncoderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockEncoder) EXPECT() *MockEncoderMockRecorder {
	return m.recorder
}

// Encode mocks base method
func (m *MockEncoder) Encode(w io.Writer, d event.Data) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Encode", w, d)
	ret0, _ := ret[0].(error)
	return ret0
}

// Encode indicates an expected call of Encode
func (mr *MockEncoderMockRecorder) Encode(w, d interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Encode", reflect.TypeOf((*MockEncoder)(nil).Encode), w, d)
}

// Decode mocks base method
func (m *MockEncoder) Decode(name string, r io.Reader) (event.Data, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Decode", name, r)
	ret0, _ := ret[0].(event.Data)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Decode indicates an expected call of Decode
func (mr *MockEncoderMockRecorder) Decode(name, r interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Decode", reflect.TypeOf((*MockEncoder)(nil).Decode), name, r)
}

// MockRegistry is a mock of Registry interface
type MockRegistry struct {
	ctrl     *gomock.Controller
	recorder *MockRegistryMockRecorder
}

// MockRegistryMockRecorder is the mock recorder for MockRegistry
type MockRegistryMockRecorder struct {
	mock *MockRegistry
}

// NewMockRegistry creates a new mock instance
func NewMockRegistry(ctrl *gomock.Controller) *MockRegistry {
	mock := &MockRegistry{ctrl: ctrl}
	mock.recorder = &MockRegistryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockRegistry) EXPECT() *MockRegistryMockRecorder {
	return m.recorder
}

// Encode mocks base method
func (m *MockRegistry) Encode(w io.Writer, d event.Data) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Encode", w, d)
	ret0, _ := ret[0].(error)
	return ret0
}

// Encode indicates an expected call of Encode
func (mr *MockRegistryMockRecorder) Encode(w, d interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Encode", reflect.TypeOf((*MockRegistry)(nil).Encode), w, d)
}

// Decode mocks base method
func (m *MockRegistry) Decode(name string, r io.Reader) (event.Data, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Decode", name, r)
	ret0, _ := ret[0].(event.Data)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Decode indicates an expected call of Decode
func (mr *MockRegistryMockRecorder) Decode(name, r interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Decode", reflect.TypeOf((*MockRegistry)(nil).Decode), name, r)
}

// Register mocks base method
func (m *MockRegistry) Register(name string, d event.Data) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Register", name, d)
}

// Register indicates an expected call of Register
func (mr *MockRegistryMockRecorder) Register(name, d interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Register", reflect.TypeOf((*MockRegistry)(nil).Register), name, d)
}
