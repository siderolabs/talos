// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.23.0
// 	protoc        v3.12.3
// source: security/security.proto

package security

import (
	context "context"
	reflect "reflect"
	sync "sync"

	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

// The request message containing the process name.
type CertificateRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Csr []byte `protobuf:"bytes,1,opt,name=csr,proto3" json:"csr,omitempty"`
}

func (x *CertificateRequest) Reset() {
	*x = CertificateRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_security_security_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CertificateRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CertificateRequest) ProtoMessage() {}

func (x *CertificateRequest) ProtoReflect() protoreflect.Message {
	mi := &file_security_security_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CertificateRequest.ProtoReflect.Descriptor instead.
func (*CertificateRequest) Descriptor() ([]byte, []int) {
	return file_security_security_proto_rawDescGZIP(), []int{0}
}

func (x *CertificateRequest) GetCsr() []byte {
	if x != nil {
		return x.Csr
	}
	return nil
}

// The response message containing the requested logs.
type CertificateResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Ca  []byte `protobuf:"bytes,1,opt,name=ca,proto3" json:"ca,omitempty"`
	Crt []byte `protobuf:"bytes,2,opt,name=crt,proto3" json:"crt,omitempty"`
}

func (x *CertificateResponse) Reset() {
	*x = CertificateResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_security_security_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CertificateResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CertificateResponse) ProtoMessage() {}

func (x *CertificateResponse) ProtoReflect() protoreflect.Message {
	mi := &file_security_security_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CertificateResponse.ProtoReflect.Descriptor instead.
func (*CertificateResponse) Descriptor() ([]byte, []int) {
	return file_security_security_proto_rawDescGZIP(), []int{1}
}

func (x *CertificateResponse) GetCa() []byte {
	if x != nil {
		return x.Ca
	}
	return nil
}

func (x *CertificateResponse) GetCrt() []byte {
	if x != nil {
		return x.Crt
	}
	return nil
}

// The request message for reading a file on disk.
type ReadFileRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Path string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
}

func (x *ReadFileRequest) Reset() {
	*x = ReadFileRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_security_security_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ReadFileRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ReadFileRequest) ProtoMessage() {}

func (x *ReadFileRequest) ProtoReflect() protoreflect.Message {
	mi := &file_security_security_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ReadFileRequest.ProtoReflect.Descriptor instead.
func (*ReadFileRequest) Descriptor() ([]byte, []int) {
	return file_security_security_proto_rawDescGZIP(), []int{2}
}

func (x *ReadFileRequest) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

// The response message for reading a file on disk.
type ReadFileResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *ReadFileResponse) Reset() {
	*x = ReadFileResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_security_security_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ReadFileResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ReadFileResponse) ProtoMessage() {}

func (x *ReadFileResponse) ProtoReflect() protoreflect.Message {
	mi := &file_security_security_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ReadFileResponse.ProtoReflect.Descriptor instead.
func (*ReadFileResponse) Descriptor() ([]byte, []int) {
	return file_security_security_proto_rawDescGZIP(), []int{3}
}

func (x *ReadFileResponse) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

// The request message containing the process name.
type WriteFileRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Path string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	Data []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	Perm int32  `protobuf:"varint,3,opt,name=perm,proto3" json:"perm,omitempty"`
}

func (x *WriteFileRequest) Reset() {
	*x = WriteFileRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_security_security_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WriteFileRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WriteFileRequest) ProtoMessage() {}

func (x *WriteFileRequest) ProtoReflect() protoreflect.Message {
	mi := &file_security_security_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WriteFileRequest.ProtoReflect.Descriptor instead.
func (*WriteFileRequest) Descriptor() ([]byte, []int) {
	return file_security_security_proto_rawDescGZIP(), []int{4}
}

func (x *WriteFileRequest) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *WriteFileRequest) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *WriteFileRequest) GetPerm() int32 {
	if x != nil {
		return x.Perm
	}
	return 0
}

// The response message containing the requested logs.
type WriteFileResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *WriteFileResponse) Reset() {
	*x = WriteFileResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_security_security_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WriteFileResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WriteFileResponse) ProtoMessage() {}

func (x *WriteFileResponse) ProtoReflect() protoreflect.Message {
	mi := &file_security_security_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WriteFileResponse.ProtoReflect.Descriptor instead.
func (*WriteFileResponse) Descriptor() ([]byte, []int) {
	return file_security_security_proto_rawDescGZIP(), []int{5}
}

var File_security_security_proto protoreflect.FileDescriptor

var file_security_security_proto_rawDesc = []byte{
	0x0a, 0x17, 0x73, 0x65, 0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x2f, 0x73, 0x65, 0x63, 0x75, 0x72,
	0x69, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0b, 0x73, 0x65, 0x63, 0x75, 0x72,
	0x69, 0x74, 0x79, 0x61, 0x70, 0x69, 0x22, 0x26, 0x0a, 0x12, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66,
	0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x10, 0x0a, 0x03,
	0x63, 0x73, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x03, 0x63, 0x73, 0x72, 0x22, 0x37,
	0x0a, 0x13, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x63, 0x61, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x02, 0x63, 0x61, 0x12, 0x10, 0x0a, 0x03, 0x63, 0x72, 0x74, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x03, 0x63, 0x72, 0x74, 0x22, 0x25, 0x0a, 0x0f, 0x52, 0x65, 0x61, 0x64, 0x46,
	0x69, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61,
	0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x61, 0x74, 0x68, 0x22, 0x26,
	0x0a, 0x10, 0x52, 0x65, 0x61, 0x64, 0x46, 0x69, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x4e, 0x0a, 0x10, 0x57, 0x72, 0x69, 0x74, 0x65, 0x46,
	0x69, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61,
	0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x61, 0x74, 0x68, 0x12, 0x12,
	0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61,
	0x74, 0x61, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x65, 0x72, 0x6d, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x04, 0x70, 0x65, 0x72, 0x6d, 0x22, 0x13, 0x0a, 0x11, 0x57, 0x72, 0x69, 0x74, 0x65, 0x46,
	0x69, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0xf8, 0x01, 0x0a, 0x0f,
	0x53, 0x65, 0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12,
	0x50, 0x0a, 0x0b, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x12, 0x1f,
	0x2e, 0x73, 0x65, 0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x61, 0x70, 0x69, 0x2e, 0x43, 0x65, 0x72,
	0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x20, 0x2e, 0x73, 0x65, 0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x61, 0x70, 0x69, 0x2e, 0x43, 0x65,
	0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x47, 0x0a, 0x08, 0x52, 0x65, 0x61, 0x64, 0x46, 0x69, 0x6c, 0x65, 0x12, 0x1c, 0x2e,
	0x73, 0x65, 0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x61, 0x70, 0x69, 0x2e, 0x52, 0x65, 0x61, 0x64,
	0x46, 0x69, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1d, 0x2e, 0x73, 0x65,
	0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x61, 0x70, 0x69, 0x2e, 0x52, 0x65, 0x61, 0x64, 0x46, 0x69,
	0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x4a, 0x0a, 0x09, 0x57, 0x72,
	0x69, 0x74, 0x65, 0x46, 0x69, 0x6c, 0x65, 0x12, 0x1d, 0x2e, 0x73, 0x65, 0x63, 0x75, 0x72, 0x69,
	0x74, 0x79, 0x61, 0x70, 0x69, 0x2e, 0x57, 0x72, 0x69, 0x74, 0x65, 0x46, 0x69, 0x6c, 0x65, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e, 0x73, 0x65, 0x63, 0x75, 0x72, 0x69, 0x74,
	0x79, 0x61, 0x70, 0x69, 0x2e, 0x57, 0x72, 0x69, 0x74, 0x65, 0x46, 0x69, 0x6c, 0x65, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x4e, 0x0a, 0x10, 0x63, 0x6f, 0x6d, 0x2e, 0x73, 0x65,
	0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x2e, 0x61, 0x70, 0x69, 0x42, 0x0b, 0x53, 0x65, 0x63, 0x75,
	0x72, 0x69, 0x74, 0x79, 0x41, 0x70, 0x69, 0x50, 0x01, 0x5a, 0x2b, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2d, 0x73, 0x79, 0x73, 0x74,
	0x65, 0x6d, 0x73, 0x2f, 0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x73, 0x65,
	0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_security_security_proto_rawDescOnce sync.Once
	file_security_security_proto_rawDescData = file_security_security_proto_rawDesc
)

func file_security_security_proto_rawDescGZIP() []byte {
	file_security_security_proto_rawDescOnce.Do(func() {
		file_security_security_proto_rawDescData = protoimpl.X.CompressGZIP(file_security_security_proto_rawDescData)
	})
	return file_security_security_proto_rawDescData
}

var (
	file_security_security_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
	file_security_security_proto_goTypes  = []interface{}{
		(*CertificateRequest)(nil),  // 0: securityapi.CertificateRequest
		(*CertificateResponse)(nil), // 1: securityapi.CertificateResponse
		(*ReadFileRequest)(nil),     // 2: securityapi.ReadFileRequest
		(*ReadFileResponse)(nil),    // 3: securityapi.ReadFileResponse
		(*WriteFileRequest)(nil),    // 4: securityapi.WriteFileRequest
		(*WriteFileResponse)(nil),   // 5: securityapi.WriteFileResponse
	}
)

var file_security_security_proto_depIdxs = []int32{
	0, // 0: securityapi.SecurityService.Certificate:input_type -> securityapi.CertificateRequest
	2, // 1: securityapi.SecurityService.ReadFile:input_type -> securityapi.ReadFileRequest
	4, // 2: securityapi.SecurityService.WriteFile:input_type -> securityapi.WriteFileRequest
	1, // 3: securityapi.SecurityService.Certificate:output_type -> securityapi.CertificateResponse
	3, // 4: securityapi.SecurityService.ReadFile:output_type -> securityapi.ReadFileResponse
	5, // 5: securityapi.SecurityService.WriteFile:output_type -> securityapi.WriteFileResponse
	3, // [3:6] is the sub-list for method output_type
	0, // [0:3] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_security_security_proto_init() }
func file_security_security_proto_init() {
	if File_security_security_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_security_security_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CertificateRequest); i {
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
		file_security_security_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CertificateResponse); i {
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
		file_security_security_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ReadFileRequest); i {
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
		file_security_security_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ReadFileResponse); i {
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
		file_security_security_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WriteFileRequest); i {
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
		file_security_security_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WriteFileResponse); i {
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
			RawDescriptor: file_security_security_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_security_security_proto_goTypes,
		DependencyIndexes: file_security_security_proto_depIdxs,
		MessageInfos:      file_security_security_proto_msgTypes,
	}.Build()
	File_security_security_proto = out.File
	file_security_security_proto_rawDesc = nil
	file_security_security_proto_goTypes = nil
	file_security_security_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ context.Context
	_ grpc.ClientConnInterface
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// SecurityServiceClient is the client API for SecurityService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type SecurityServiceClient interface {
	Certificate(ctx context.Context, in *CertificateRequest, opts ...grpc.CallOption) (*CertificateResponse, error)
	ReadFile(ctx context.Context, in *ReadFileRequest, opts ...grpc.CallOption) (*ReadFileResponse, error)
	WriteFile(ctx context.Context, in *WriteFileRequest, opts ...grpc.CallOption) (*WriteFileResponse, error)
}

type securityServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSecurityServiceClient(cc grpc.ClientConnInterface) SecurityServiceClient {
	return &securityServiceClient{cc}
}

func (c *securityServiceClient) Certificate(ctx context.Context, in *CertificateRequest, opts ...grpc.CallOption) (*CertificateResponse, error) {
	out := new(CertificateResponse)
	err := c.cc.Invoke(ctx, "/securityapi.SecurityService/Certificate", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *securityServiceClient) ReadFile(ctx context.Context, in *ReadFileRequest, opts ...grpc.CallOption) (*ReadFileResponse, error) {
	out := new(ReadFileResponse)
	err := c.cc.Invoke(ctx, "/securityapi.SecurityService/ReadFile", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *securityServiceClient) WriteFile(ctx context.Context, in *WriteFileRequest, opts ...grpc.CallOption) (*WriteFileResponse, error) {
	out := new(WriteFileResponse)
	err := c.cc.Invoke(ctx, "/securityapi.SecurityService/WriteFile", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SecurityServiceServer is the server API for SecurityService service.
type SecurityServiceServer interface {
	Certificate(context.Context, *CertificateRequest) (*CertificateResponse, error)
	ReadFile(context.Context, *ReadFileRequest) (*ReadFileResponse, error)
	WriteFile(context.Context, *WriteFileRequest) (*WriteFileResponse, error)
}

// UnimplementedSecurityServiceServer can be embedded to have forward compatible implementations.
type UnimplementedSecurityServiceServer struct {
}

func (*UnimplementedSecurityServiceServer) Certificate(context.Context, *CertificateRequest) (*CertificateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Certificate not implemented")
}

func (*UnimplementedSecurityServiceServer) ReadFile(context.Context, *ReadFileRequest) (*ReadFileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadFile not implemented")
}

func (*UnimplementedSecurityServiceServer) WriteFile(context.Context, *WriteFileRequest) (*WriteFileResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method WriteFile not implemented")
}

func RegisterSecurityServiceServer(s *grpc.Server, srv SecurityServiceServer) {
	s.RegisterService(&_SecurityService_serviceDesc, srv)
}

func _SecurityService_Certificate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CertificateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecurityServiceServer).Certificate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/securityapi.SecurityService/Certificate",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecurityServiceServer).Certificate(ctx, req.(*CertificateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SecurityService_ReadFile_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReadFileRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecurityServiceServer).ReadFile(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/securityapi.SecurityService/ReadFile",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecurityServiceServer).ReadFile(ctx, req.(*ReadFileRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SecurityService_WriteFile_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(WriteFileRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SecurityServiceServer).WriteFile(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/securityapi.SecurityService/WriteFile",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SecurityServiceServer).WriteFile(ctx, req.(*WriteFileRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _SecurityService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "securityapi.SecurityService",
	HandlerType: (*SecurityServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Certificate",
			Handler:    _SecurityService_Certificate_Handler,
		},
		{
			MethodName: "ReadFile",
			Handler:    _SecurityService_ReadFile_Handler,
		},
		{
			MethodName: "WriteFile",
			Handler:    _SecurityService_WriteFile_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "security/security.proto",
}
