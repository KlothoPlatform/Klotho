// Code generated by MockGen. DO NOT EDIT.
// Source: ./resource_template.go
//
// Generated by this command:
//
//	mockgen -source=./resource_template.go --destination=../engine2/operational_eval/resource_template_mock_test.go --package=operational_eval
//

// Package operational_eval is a generated GoMock package.
package operational_eval

import (
	reflect "reflect"

	construct "github.com/klothoplatform/klotho/pkg/construct"
	knowledgebase "github.com/klothoplatform/klotho/pkg/knowledgebase"
	gomock "go.uber.org/mock/gomock"
)

// MockProperty is a mock of Property interface.
type MockProperty struct {
	ctrl     *gomock.Controller
	recorder *MockPropertyMockRecorder
}

// MockPropertyMockRecorder is the mock recorder for MockProperty.
type MockPropertyMockRecorder struct {
	mock *MockProperty
}

// NewMockProperty creates a new mock instance.
func NewMockProperty(ctrl *gomock.Controller) *MockProperty {
	mock := &MockProperty{ctrl: ctrl}
	mock.recorder = &MockPropertyMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockProperty) EXPECT() *MockPropertyMockRecorder {
	return m.recorder
}

// AppendProperty mocks base method.
func (m *MockProperty) AppendProperty(resource *construct.Resource, value any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AppendProperty", resource, value)
	ret0, _ := ret[0].(error)
	return ret0
}

// AppendProperty indicates an expected call of AppendProperty.
func (mr *MockPropertyMockRecorder) AppendProperty(resource, value any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AppendProperty", reflect.TypeOf((*MockProperty)(nil).AppendProperty), resource, value)
}

// Clone mocks base method.
func (m *MockProperty) Clone() knowledgebase.Property {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Clone")
	ret0, _ := ret[0].(knowledgebase.Property)
	return ret0
}

// Clone indicates an expected call of Clone.
func (mr *MockPropertyMockRecorder) Clone() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Clone", reflect.TypeOf((*MockProperty)(nil).Clone))
}

// Contains mocks base method.
func (m *MockProperty) Contains(value, contains any) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Contains", value, contains)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Contains indicates an expected call of Contains.
func (mr *MockPropertyMockRecorder) Contains(value, contains any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Contains", reflect.TypeOf((*MockProperty)(nil).Contains), value, contains)
}

// Details mocks base method.
func (m *MockProperty) Details() *knowledgebase.PropertyDetails {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Details")
	ret0, _ := ret[0].(*knowledgebase.PropertyDetails)
	return ret0
}

// Details indicates an expected call of Details.
func (mr *MockPropertyMockRecorder) Details() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Details", reflect.TypeOf((*MockProperty)(nil).Details))
}

// GetDefaultValue mocks base method.
func (m *MockProperty) GetDefaultValue(ctx knowledgebase.DynamicContext, data knowledgebase.DynamicValueData) (any, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetDefaultValue", ctx, data)
	ret0, _ := ret[0].(any)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetDefaultValue indicates an expected call of GetDefaultValue.
func (mr *MockPropertyMockRecorder) GetDefaultValue(ctx, data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDefaultValue", reflect.TypeOf((*MockProperty)(nil).GetDefaultValue), ctx, data)
}

// Parse mocks base method.
func (m *MockProperty) Parse(value any, ctx knowledgebase.DynamicContext, data knowledgebase.DynamicValueData) (any, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Parse", value, ctx, data)
	ret0, _ := ret[0].(any)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Parse indicates an expected call of Parse.
func (mr *MockPropertyMockRecorder) Parse(value, ctx, data any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Parse", reflect.TypeOf((*MockProperty)(nil).Parse), value, ctx, data)
}

// RemoveProperty mocks base method.
func (m *MockProperty) RemoveProperty(resource *construct.Resource, value any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveProperty", resource, value)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveProperty indicates an expected call of RemoveProperty.
func (mr *MockPropertyMockRecorder) RemoveProperty(resource, value any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveProperty", reflect.TypeOf((*MockProperty)(nil).RemoveProperty), resource, value)
}

// SetProperty mocks base method.
func (m *MockProperty) SetProperty(resource *construct.Resource, value any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetProperty", resource, value)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetProperty indicates an expected call of SetProperty.
func (mr *MockPropertyMockRecorder) SetProperty(resource, value any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetProperty", reflect.TypeOf((*MockProperty)(nil).SetProperty), resource, value)
}

// SubProperties mocks base method.
func (m *MockProperty) SubProperties() knowledgebase.Properties {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubProperties")
	ret0, _ := ret[0].(knowledgebase.Properties)
	return ret0
}

// SubProperties indicates an expected call of SubProperties.
func (mr *MockPropertyMockRecorder) SubProperties() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubProperties", reflect.TypeOf((*MockProperty)(nil).SubProperties))
}

// Type mocks base method.
func (m *MockProperty) Type() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Type")
	ret0, _ := ret[0].(string)
	return ret0
}

// Type indicates an expected call of Type.
func (mr *MockPropertyMockRecorder) Type() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Type", reflect.TypeOf((*MockProperty)(nil).Type))
}

// Validate mocks base method.
func (m *MockProperty) Validate(resource *construct.Resource, value any, ctx knowledgebase.DynamicContext) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Validate", resource, value, ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// Validate indicates an expected call of Validate.
func (mr *MockPropertyMockRecorder) Validate(resource, value, ctx any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Validate", reflect.TypeOf((*MockProperty)(nil).Validate), resource, value, ctx)
}

// ZeroValue mocks base method.
func (m *MockProperty) ZeroValue() any {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ZeroValue")
	ret0, _ := ret[0].(any)
	return ret0
}

// ZeroValue indicates an expected call of ZeroValue.
func (mr *MockPropertyMockRecorder) ZeroValue() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ZeroValue", reflect.TypeOf((*MockProperty)(nil).ZeroValue))
}

// MockMapProperty is a mock of MapProperty interface.
type MockMapProperty struct {
	ctrl     *gomock.Controller
	recorder *MockMapPropertyMockRecorder
}

// MockMapPropertyMockRecorder is the mock recorder for MockMapProperty.
type MockMapPropertyMockRecorder struct {
	mock *MockMapProperty
}

// NewMockMapProperty creates a new mock instance.
func NewMockMapProperty(ctrl *gomock.Controller) *MockMapProperty {
	mock := &MockMapProperty{ctrl: ctrl}
	mock.recorder = &MockMapPropertyMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMapProperty) EXPECT() *MockMapPropertyMockRecorder {
	return m.recorder
}

// Key mocks base method.
func (m *MockMapProperty) Key() knowledgebase.Property {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Key")
	ret0, _ := ret[0].(knowledgebase.Property)
	return ret0
}

// Key indicates an expected call of Key.
func (mr *MockMapPropertyMockRecorder) Key() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Key", reflect.TypeOf((*MockMapProperty)(nil).Key))
}

// Value mocks base method.
func (m *MockMapProperty) Value() knowledgebase.Property {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Value")
	ret0, _ := ret[0].(knowledgebase.Property)
	return ret0
}

// Value indicates an expected call of Value.
func (mr *MockMapPropertyMockRecorder) Value() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Value", reflect.TypeOf((*MockMapProperty)(nil).Value))
}

// MockCollectionProperty is a mock of CollectionProperty interface.
type MockCollectionProperty struct {
	ctrl     *gomock.Controller
	recorder *MockCollectionPropertyMockRecorder
}

// MockCollectionPropertyMockRecorder is the mock recorder for MockCollectionProperty.
type MockCollectionPropertyMockRecorder struct {
	mock *MockCollectionProperty
}

// NewMockCollectionProperty creates a new mock instance.
func NewMockCollectionProperty(ctrl *gomock.Controller) *MockCollectionProperty {
	mock := &MockCollectionProperty{ctrl: ctrl}
	mock.recorder = &MockCollectionPropertyMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCollectionProperty) EXPECT() *MockCollectionPropertyMockRecorder {
	return m.recorder
}

// Item mocks base method.
func (m *MockCollectionProperty) Item() knowledgebase.Property {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Item")
	ret0, _ := ret[0].(knowledgebase.Property)
	return ret0
}

// Item indicates an expected call of Item.
func (mr *MockCollectionPropertyMockRecorder) Item() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Item", reflect.TypeOf((*MockCollectionProperty)(nil).Item))
}
