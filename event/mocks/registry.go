// Code generated by MockGen. DO NOT EDIT.
// Source: registry.go

// Package mock_event is a generated GoMock package.
package mock_event

import (
	io "io"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	event "github.com/modernice/goes/event"
)

// MockEncoder is a mock of Encoder interface.
type MockEncoder struct {
	ctrl     *gomock.Controller
	recorder *MockEncoderMockRecorder
}

// MockEncoderMockRecorder is the mock recorder for MockEncoder.
type MockEncoderMockRecorder struct {
	mock *MockEncoder
}

// NewMockEncoder creates a new mock instance.
func NewMockEncoder(ctrl *gomock.Controller) *MockEncoder {
	mock := &MockEncoder{ctrl: ctrl}
	mock.recorder = &MockEncoderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEncoder) EXPECT() *MockEncoderMockRecorder {
	return m.recorder
}

// Decode mocks base method.
func (m *MockEncoder) Decode(name string, r io.Reader) (event.Data, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Decode", name, r)
	ret0, _ := ret[0].(event.Data)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Decode indicates an expected call of Decode.
func (mr *MockEncoderMockRecorder) Decode(name, r interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Decode", reflect.TypeOf((*MockEncoder)(nil).Decode), name, r)
}

// Encode mocks base method.
func (m *MockEncoder) Encode(w io.Writer, name string, d event.Data) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Encode", w, name, d)
	ret0, _ := ret[0].(error)
	return ret0
}

// Encode indicates an expected call of Encode.
func (mr *MockEncoderMockRecorder) Encode(w, name, d interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Encode", reflect.TypeOf((*MockEncoder)(nil).Encode), w, name, d)
}

// MockRegistry is a mock of Registry interface.
type MockRegistry struct {
	ctrl     *gomock.Controller
	recorder *MockRegistryMockRecorder
}

// MockRegistryMockRecorder is the mock recorder for MockRegistry.
type MockRegistryMockRecorder struct {
	mock *MockRegistry
}

// NewMockRegistry creates a new mock instance.
func NewMockRegistry(ctrl *gomock.Controller) *MockRegistry {
	mock := &MockRegistry{ctrl: ctrl}
	mock.recorder = &MockRegistryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRegistry) EXPECT() *MockRegistryMockRecorder {
	return m.recorder
}

// Decode mocks base method.
func (m *MockRegistry) Decode(name string, r io.Reader) (event.Data, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Decode", name, r)
	ret0, _ := ret[0].(event.Data)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Decode indicates an expected call of Decode.
func (mr *MockRegistryMockRecorder) Decode(name, r interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Decode", reflect.TypeOf((*MockRegistry)(nil).Decode), name, r)
}

// Encode mocks base method.
func (m *MockRegistry) Encode(w io.Writer, name string, d event.Data) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Encode", w, name, d)
	ret0, _ := ret[0].(error)
	return ret0
}

// Encode indicates an expected call of Encode.
func (mr *MockRegistryMockRecorder) Encode(w, name, d interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Encode", reflect.TypeOf((*MockRegistry)(nil).Encode), w, name, d)
}

// New mocks base method.
func (m *MockRegistry) New(name string) (event.Data, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "New", name)
	ret0, _ := ret[0].(event.Data)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// New indicates an expected call of New.
func (mr *MockRegistryMockRecorder) New(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "New", reflect.TypeOf((*MockRegistry)(nil).New), name)
}

// Register mocks base method.
func (m *MockRegistry) Register(name string, newData func() event.Data) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Register", name, newData)
}

// Register indicates an expected call of Register.
func (mr *MockRegistryMockRecorder) Register(name, newData interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Register", reflect.TypeOf((*MockRegistry)(nil).Register), name, newData)
}

// MockMarshaler is a mock of Marshaler interface.
type MockMarshaler struct {
	ctrl     *gomock.Controller
	recorder *MockMarshalerMockRecorder
}

// MockMarshalerMockRecorder is the mock recorder for MockMarshaler.
type MockMarshalerMockRecorder struct {
	mock *MockMarshaler
}

// NewMockMarshaler creates a new mock instance.
func NewMockMarshaler(ctrl *gomock.Controller) *MockMarshaler {
	mock := &MockMarshaler{ctrl: ctrl}
	mock.recorder = &MockMarshalerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMarshaler) EXPECT() *MockMarshalerMockRecorder {
	return m.recorder
}

// MarshalEvent mocks base method.
func (m *MockMarshaler) MarshalEvent() ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MarshalEvent")
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// MarshalEvent indicates an expected call of MarshalEvent.
func (mr *MockMarshalerMockRecorder) MarshalEvent() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MarshalEvent", reflect.TypeOf((*MockMarshaler)(nil).MarshalEvent))
}

// MockUnmarshaler is a mock of Unmarshaler interface.
type MockUnmarshaler struct {
	ctrl     *gomock.Controller
	recorder *MockUnmarshalerMockRecorder
}

// MockUnmarshalerMockRecorder is the mock recorder for MockUnmarshaler.
type MockUnmarshalerMockRecorder struct {
	mock *MockUnmarshaler
}

// NewMockUnmarshaler creates a new mock instance.
func NewMockUnmarshaler(ctrl *gomock.Controller) *MockUnmarshaler {
	mock := &MockUnmarshaler{ctrl: ctrl}
	mock.recorder = &MockUnmarshalerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockUnmarshaler) EXPECT() *MockUnmarshalerMockRecorder {
	return m.recorder
}

// UnmarshalEvent mocks base method.
func (m *MockUnmarshaler) UnmarshalEvent(arg0 []byte) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UnmarshalEvent", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// UnmarshalEvent indicates an expected call of UnmarshalEvent.
func (mr *MockUnmarshalerMockRecorder) UnmarshalEvent(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UnmarshalEvent", reflect.TypeOf((*MockUnmarshaler)(nil).UnmarshalEvent), arg0)
}
