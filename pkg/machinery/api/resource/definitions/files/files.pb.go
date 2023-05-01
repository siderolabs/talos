// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v4.22.2
// source: resource/definitions/files/files.proto

package files

import (
	reflect "reflect"
	sync "sync"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// EtcFileSpecSpec describes status of rendered secrets.
type EtcFileSpecSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Contents []byte `protobuf:"bytes,1,opt,name=contents,proto3" json:"contents,omitempty"`
	Mode     uint32 `protobuf:"varint,2,opt,name=mode,proto3" json:"mode,omitempty"`
}

func (x *EtcFileSpecSpec) Reset() {
	*x = EtcFileSpecSpec{}
	if protoimpl.UnsafeEnabled {
		mi := &file_resource_definitions_files_files_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EtcFileSpecSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EtcFileSpecSpec) ProtoMessage() {}

func (x *EtcFileSpecSpec) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_files_files_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EtcFileSpecSpec.ProtoReflect.Descriptor instead.
func (*EtcFileSpecSpec) Descriptor() ([]byte, []int) {
	return file_resource_definitions_files_files_proto_rawDescGZIP(), []int{0}
}

func (x *EtcFileSpecSpec) GetContents() []byte {
	if x != nil {
		return x.Contents
	}
	return nil
}

func (x *EtcFileSpecSpec) GetMode() uint32 {
	if x != nil {
		return x.Mode
	}
	return 0
}

// EtcFileStatusSpec describes status of rendered secrets.
type EtcFileStatusSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SpecVersion string `protobuf:"bytes,1,opt,name=spec_version,json=specVersion,proto3" json:"spec_version,omitempty"`
}

func (x *EtcFileStatusSpec) Reset() {
	*x = EtcFileStatusSpec{}
	if protoimpl.UnsafeEnabled {
		mi := &file_resource_definitions_files_files_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EtcFileStatusSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EtcFileStatusSpec) ProtoMessage() {}

func (x *EtcFileStatusSpec) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_files_files_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EtcFileStatusSpec.ProtoReflect.Descriptor instead.
func (*EtcFileStatusSpec) Descriptor() ([]byte, []int) {
	return file_resource_definitions_files_files_proto_rawDescGZIP(), []int{1}
}

func (x *EtcFileStatusSpec) GetSpecVersion() string {
	if x != nil {
		return x.SpecVersion
	}
	return ""
}

// UdevRuleSpec is the specification for UdevRule resource.
type UdevRuleSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Rule string `protobuf:"bytes,1,opt,name=rule,proto3" json:"rule,omitempty"`
}

func (x *UdevRuleSpec) Reset() {
	*x = UdevRuleSpec{}
	if protoimpl.UnsafeEnabled {
		mi := &file_resource_definitions_files_files_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UdevRuleSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UdevRuleSpec) ProtoMessage() {}

func (x *UdevRuleSpec) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_files_files_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UdevRuleSpec.ProtoReflect.Descriptor instead.
func (*UdevRuleSpec) Descriptor() ([]byte, []int) {
	return file_resource_definitions_files_files_proto_rawDescGZIP(), []int{2}
}

func (x *UdevRuleSpec) GetRule() string {
	if x != nil {
		return x.Rule
	}
	return ""
}

// UdevRuleStatusSpec is the specification for UdevRule resource.
type UdevRuleStatusSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Active bool `protobuf:"varint,1,opt,name=active,proto3" json:"active,omitempty"`
}

func (x *UdevRuleStatusSpec) Reset() {
	*x = UdevRuleStatusSpec{}
	if protoimpl.UnsafeEnabled {
		mi := &file_resource_definitions_files_files_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UdevRuleStatusSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UdevRuleStatusSpec) ProtoMessage() {}

func (x *UdevRuleStatusSpec) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_files_files_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UdevRuleStatusSpec.ProtoReflect.Descriptor instead.
func (*UdevRuleStatusSpec) Descriptor() ([]byte, []int) {
	return file_resource_definitions_files_files_proto_rawDescGZIP(), []int{3}
}

func (x *UdevRuleStatusSpec) GetActive() bool {
	if x != nil {
		return x.Active
	}
	return false
}

var File_resource_definitions_files_files_proto protoreflect.FileDescriptor

var file_resource_definitions_files_files_proto_rawDesc = []byte{
	0x0a, 0x26, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2f, 0x64, 0x65, 0x66, 0x69, 0x6e,
	0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2f, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x2f, 0x66, 0x69, 0x6c,
	0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x20, 0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2e,
	0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x22, 0x41, 0x0a, 0x0f, 0x45, 0x74,
	0x63, 0x46, 0x69, 0x6c, 0x65, 0x53, 0x70, 0x65, 0x63, 0x53, 0x70, 0x65, 0x63, 0x12, 0x1a, 0x0a,
	0x08, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x08, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x6d, 0x6f, 0x64,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x04, 0x6d, 0x6f, 0x64, 0x65, 0x22, 0x36, 0x0a,
	0x11, 0x45, 0x74, 0x63, 0x46, 0x69, 0x6c, 0x65, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x53, 0x70,
	0x65, 0x63, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x70, 0x65, 0x63, 0x5f, 0x76, 0x65, 0x72, 0x73, 0x69,
	0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x73, 0x70, 0x65, 0x63, 0x56, 0x65,
	0x72, 0x73, 0x69, 0x6f, 0x6e, 0x22, 0x22, 0x0a, 0x0c, 0x55, 0x64, 0x65, 0x76, 0x52, 0x75, 0x6c,
	0x65, 0x53, 0x70, 0x65, 0x63, 0x12, 0x12, 0x0a, 0x04, 0x72, 0x75, 0x6c, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x72, 0x75, 0x6c, 0x65, 0x22, 0x2c, 0x0a, 0x12, 0x55, 0x64, 0x65,
	0x76, 0x52, 0x75, 0x6c, 0x65, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x53, 0x70, 0x65, 0x63, 0x12,
	0x16, 0x0a, 0x06, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52,
	0x06, 0x61, 0x63, 0x74, 0x69, 0x76, 0x65, 0x42, 0x4a, 0x5a, 0x48, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x69, 0x64, 0x65, 0x72, 0x6f, 0x6c, 0x61, 0x62, 0x73,
	0x2f, 0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x6d, 0x61, 0x63, 0x68, 0x69,
	0x6e, 0x65, 0x72, 0x79, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63,
	0x65, 0x2f, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2f, 0x66, 0x69,
	0x6c, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_resource_definitions_files_files_proto_rawDescOnce sync.Once
	file_resource_definitions_files_files_proto_rawDescData = file_resource_definitions_files_files_proto_rawDesc
)

func file_resource_definitions_files_files_proto_rawDescGZIP() []byte {
	file_resource_definitions_files_files_proto_rawDescOnce.Do(func() {
		file_resource_definitions_files_files_proto_rawDescData = protoimpl.X.CompressGZIP(file_resource_definitions_files_files_proto_rawDescData)
	})
	return file_resource_definitions_files_files_proto_rawDescData
}

var file_resource_definitions_files_files_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_resource_definitions_files_files_proto_goTypes = []interface{}{
	(*EtcFileSpecSpec)(nil),    // 0: talos.resource.definitions.files.EtcFileSpecSpec
	(*EtcFileStatusSpec)(nil),  // 1: talos.resource.definitions.files.EtcFileStatusSpec
	(*UdevRuleSpec)(nil),       // 2: talos.resource.definitions.files.UdevRuleSpec
	(*UdevRuleStatusSpec)(nil), // 3: talos.resource.definitions.files.UdevRuleStatusSpec
}
var file_resource_definitions_files_files_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_resource_definitions_files_files_proto_init() }
func file_resource_definitions_files_files_proto_init() {
	if File_resource_definitions_files_files_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_resource_definitions_files_files_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EtcFileSpecSpec); i {
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
		file_resource_definitions_files_files_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EtcFileStatusSpec); i {
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
		file_resource_definitions_files_files_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UdevRuleSpec); i {
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
		file_resource_definitions_files_files_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UdevRuleStatusSpec); i {
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
			RawDescriptor: file_resource_definitions_files_files_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_resource_definitions_files_files_proto_goTypes,
		DependencyIndexes: file_resource_definitions_files_files_proto_depIdxs,
		MessageInfos:      file_resource_definitions_files_files_proto_msgTypes,
	}.Build()
	File_resource_definitions_files_files_proto = out.File
	file_resource_definitions_files_files_proto_rawDesc = nil
	file_resource_definitions_files_files_proto_goTypes = nil
	file_resource_definitions_files_files_proto_depIdxs = nil
}
