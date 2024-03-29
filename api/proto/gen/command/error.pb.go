// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.15.3
// source: goes/command/error.proto

package commandpb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	anypb "google.golang.org/protobuf/types/known/anypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Error represents an error response from a command execution. It contains an
// error code, a message describing the error, and a list of ErrorDetails
// [ErrorDetail].
type Error struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Code    int64          `protobuf:"varint,1,opt,name=code,proto3" json:"code,omitempty"`
	Message string         `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	Details []*ErrorDetail `protobuf:"bytes,3,rep,name=details,proto3" json:"details,omitempty"`
}

// Reset resets the Error to its zero value. It also resets the message state if
// unsafe operations are enabled.
func (x *Error) Reset() {
	*x = Error{}
	if protoimpl.UnsafeEnabled {
		mi := &file_goes_command_error_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

// String returns a string representation of the Error message. It is generated
// by the protoimpl library and uses Go's standard fmt package to format the
// message fields.
func (x *Error) String() string {
	return protoimpl.X.MessageStringOf(x)
}

// ProtoMessage *Error.ProtoMessage* is a method of the Error struct in the
// goes/commandpb package. It is implemented as an empty method and is part of
// the protoreflect.Message interface.
func (*Error) ProtoMessage() {}

// ProtoReflect returns a protoreflect.Message representing the receiver. This
// method is used internally by the proto package and should not be called
// directly.
func (x *Error) ProtoReflect() protoreflect.Message {
	mi := &file_goes_command_error_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Error.ProtoReflect.Descriptor instead.
func (*Error) Descriptor() ([]byte, []int) {
	return file_goes_command_error_proto_rawDescGZIP(), []int{0}
}

// GetCode returns the error code of an Error message.
func (x *Error) GetCode() int64 {
	if x != nil {
		return x.Code
	}
	return 0
}

// GetMessage returns the error message string of an Error type [Error]. If the
// Error is nil, an empty string is returned.
func (x *Error) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

// GetDetails returns a slice of *ErrorDetail containing details about the
// error. If there are no details, it returns nil.
func (x *Error) GetDetails() []*ErrorDetail {
	if x != nil {
		return x.Details
	}
	return nil
}

// ErrorDetail represents a single detail of an error. It contains a field named
// Detail that is of type
// [anypb.Any](https://pkg.go.dev/google.golang.org/protobuf/types/known/anypb#Any).
type ErrorDetail struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Detail *anypb.Any `protobuf:"bytes,1,opt,name=detail,proto3" json:"detail,omitempty"`
}

// Reset resets the ErrorDetail to its zero value.
func (x *ErrorDetail) Reset() {
	*x = ErrorDetail{}
	if protoimpl.UnsafeEnabled {
		mi := &file_goes_command_error_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

// String returns a string representation of the ErrorDetail message. It uses
// protoimpl.X.MessageStringOf to generate its output.
func (x *ErrorDetail) String() string {
	return protoimpl.X.MessageStringOf(x)
}

// ProtoMessage is a method implemented by ErrorDetail that marks the struct as
// implementing the protoreflect.ProtoMessage interface. This is required for
// any struct that needs to be serialized or deserialized using Protocol
// Buffers.
func (*ErrorDetail) ProtoMessage() {}

// ProtoReflect returns the message's reflection interface, which is used to
// manipulate protobuf messages dynamically. It returns a protoreflect.Message
// that implements the Message interface in package
// [google.golang.org/protobuf/reflect/protoreflect](https://godoc.org/google.golang.org/protobuf/reflect/protoreflect).
func (x *ErrorDetail) ProtoReflect() protoreflect.Message {
	mi := &file_goes_command_error_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ErrorDetail.ProtoReflect.Descriptor instead.
func (*ErrorDetail) Descriptor() ([]byte, []int) {
	return file_goes_command_error_proto_rawDescGZIP(), []int{1}
}

// GetDetail returns the Any message contained in the ErrorDetail. It returns
// nil if the ErrorDetail is nil.
func (x *ErrorDetail) GetDetail() *anypb.Any {
	if x != nil {
		return x.Detail
	}
	return nil
}

// File_goes_command_error_proto defines the Error and ErrorDetail message types
// used for representing errors in command execution. The Error message type
// contains an integer code, a string message, and a repeated field of
// ErrorDetail messages. The ErrorDetail message type contains a
// google.protobuf.Any field named "detail".
var File_goes_command_error_proto protoreflect.FileDescriptor

var file_goes_command_error_proto_rawDesc = []byte{
	0x0a, 0x18, 0x67, 0x6f, 0x65, 0x73, 0x2f, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x2f, 0x65,
	0x72, 0x72, 0x6f, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0c, 0x67, 0x6f, 0x65, 0x73,
	0x2e, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x1a, 0x19, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x61, 0x6e, 0x79, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0x6a, 0x0a, 0x05, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x12, 0x12, 0x0a, 0x04,
	0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65,
	0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x33, 0x0a, 0x07, 0x64, 0x65,
	0x74, 0x61, 0x69, 0x6c, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f,
	0x65, 0x73, 0x2e, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x2e, 0x45, 0x72, 0x72, 0x6f, 0x72,
	0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x52, 0x07, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x22,
	0x3b, 0x0a, 0x0b, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x44, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x12, 0x2c,
	0x0a, 0x06, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x41, 0x6e, 0x79, 0x52, 0x06, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x42, 0x3b, 0x5a, 0x39,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6d, 0x6f, 0x64, 0x65, 0x72,
	0x6e, 0x69, 0x63, 0x65, 0x2f, 0x67, 0x6f, 0x65, 0x73, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x65, 0x6e, 0x2f, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x3b,
	0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_goes_command_error_proto_rawDescOnce sync.Once
	file_goes_command_error_proto_rawDescData = file_goes_command_error_proto_rawDesc
)

func file_goes_command_error_proto_rawDescGZIP() []byte {
	file_goes_command_error_proto_rawDescOnce.Do(func() {
		file_goes_command_error_proto_rawDescData = protoimpl.X.CompressGZIP(file_goes_command_error_proto_rawDescData)
	})
	return file_goes_command_error_proto_rawDescData
}

var file_goes_command_error_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_goes_command_error_proto_goTypes = []interface{}{
	(*Error)(nil),       // 0: goes.command.Error
	(*ErrorDetail)(nil), // 1: goes.command.ErrorDetail
	(*anypb.Any)(nil),   // 2: google.protobuf.Any
}
var file_goes_command_error_proto_depIdxs = []int32{
	1, // 0: goes.command.Error.details:type_name -> goes.command.ErrorDetail
	2, // 1: goes.command.ErrorDetail.detail:type_name -> google.protobuf.Any
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_goes_command_error_proto_init() }
func file_goes_command_error_proto_init() {
	if File_goes_command_error_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_goes_command_error_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Error); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_goes_command_error_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ErrorDetail); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_goes_command_error_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_goes_command_error_proto_goTypes,
		DependencyIndexes: file_goes_command_error_proto_depIdxs,
		MessageInfos:      file_goes_command_error_proto_msgTypes,
	}.Build()
	File_goes_command_error_proto = out.File
	file_goes_command_error_proto_rawDesc = nil
	file_goes_command_error_proto_goTypes = nil
	file_goes_command_error_proto_depIdxs = nil
}
