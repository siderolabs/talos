// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v6.31.1
// source: resource/definitions/time/time.proto

package time

import (
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// AdjtimeStatusSpec describes Linux internal adjtime state.
type AdjtimeStatusSpec struct {
	state                    protoimpl.MessageState `protogen:"open.v1"`
	Offset                   *durationpb.Duration   `protobuf:"bytes,1,opt,name=offset,proto3" json:"offset,omitempty"`
	FrequencyAdjustmentRatio float64                `protobuf:"fixed64,2,opt,name=frequency_adjustment_ratio,json=frequencyAdjustmentRatio,proto3" json:"frequency_adjustment_ratio,omitempty"`
	MaxError                 *durationpb.Duration   `protobuf:"bytes,3,opt,name=max_error,json=maxError,proto3" json:"max_error,omitempty"`
	EstError                 *durationpb.Duration   `protobuf:"bytes,4,opt,name=est_error,json=estError,proto3" json:"est_error,omitempty"`
	Status                   string                 `protobuf:"bytes,5,opt,name=status,proto3" json:"status,omitempty"`
	Constant                 int64                  `protobuf:"varint,6,opt,name=constant,proto3" json:"constant,omitempty"`
	SyncStatus               bool                   `protobuf:"varint,7,opt,name=sync_status,json=syncStatus,proto3" json:"sync_status,omitempty"`
	State                    string                 `protobuf:"bytes,8,opt,name=state,proto3" json:"state,omitempty"`
	unknownFields            protoimpl.UnknownFields
	sizeCache                protoimpl.SizeCache
}

func (x *AdjtimeStatusSpec) Reset() {
	*x = AdjtimeStatusSpec{}
	mi := &file_resource_definitions_time_time_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AdjtimeStatusSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AdjtimeStatusSpec) ProtoMessage() {}

func (x *AdjtimeStatusSpec) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_time_time_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AdjtimeStatusSpec.ProtoReflect.Descriptor instead.
func (*AdjtimeStatusSpec) Descriptor() ([]byte, []int) {
	return file_resource_definitions_time_time_proto_rawDescGZIP(), []int{0}
}

func (x *AdjtimeStatusSpec) GetOffset() *durationpb.Duration {
	if x != nil {
		return x.Offset
	}
	return nil
}

func (x *AdjtimeStatusSpec) GetFrequencyAdjustmentRatio() float64 {
	if x != nil {
		return x.FrequencyAdjustmentRatio
	}
	return 0
}

func (x *AdjtimeStatusSpec) GetMaxError() *durationpb.Duration {
	if x != nil {
		return x.MaxError
	}
	return nil
}

func (x *AdjtimeStatusSpec) GetEstError() *durationpb.Duration {
	if x != nil {
		return x.EstError
	}
	return nil
}

func (x *AdjtimeStatusSpec) GetStatus() string {
	if x != nil {
		return x.Status
	}
	return ""
}

func (x *AdjtimeStatusSpec) GetConstant() int64 {
	if x != nil {
		return x.Constant
	}
	return 0
}

func (x *AdjtimeStatusSpec) GetSyncStatus() bool {
	if x != nil {
		return x.SyncStatus
	}
	return false
}

func (x *AdjtimeStatusSpec) GetState() string {
	if x != nil {
		return x.State
	}
	return ""
}

// StatusSpec describes time sync state.
type StatusSpec struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Synced        bool                   `protobuf:"varint,1,opt,name=synced,proto3" json:"synced,omitempty"`
	Epoch         int64                  `protobuf:"varint,2,opt,name=epoch,proto3" json:"epoch,omitempty"`
	SyncDisabled  bool                   `protobuf:"varint,3,opt,name=sync_disabled,json=syncDisabled,proto3" json:"sync_disabled,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *StatusSpec) Reset() {
	*x = StatusSpec{}
	mi := &file_resource_definitions_time_time_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *StatusSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StatusSpec) ProtoMessage() {}

func (x *StatusSpec) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_time_time_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StatusSpec.ProtoReflect.Descriptor instead.
func (*StatusSpec) Descriptor() ([]byte, []int) {
	return file_resource_definitions_time_time_proto_rawDescGZIP(), []int{1}
}

func (x *StatusSpec) GetSynced() bool {
	if x != nil {
		return x.Synced
	}
	return false
}

func (x *StatusSpec) GetEpoch() int64 {
	if x != nil {
		return x.Epoch
	}
	return 0
}

func (x *StatusSpec) GetSyncDisabled() bool {
	if x != nil {
		return x.SyncDisabled
	}
	return false
}

var File_resource_definitions_time_time_proto protoreflect.FileDescriptor

const file_resource_definitions_time_time_proto_rawDesc = "" +
	"\n" +
	"$resource/definitions/time/time.proto\x12\x1ftalos.resource.definitions.time\x1a\x1egoogle/protobuf/duration.proto\"\xdf\x02\n" +
	"\x11AdjtimeStatusSpec\x121\n" +
	"\x06offset\x18\x01 \x01(\v2\x19.google.protobuf.DurationR\x06offset\x12<\n" +
	"\x1afrequency_adjustment_ratio\x18\x02 \x01(\x01R\x18frequencyAdjustmentRatio\x126\n" +
	"\tmax_error\x18\x03 \x01(\v2\x19.google.protobuf.DurationR\bmaxError\x126\n" +
	"\test_error\x18\x04 \x01(\v2\x19.google.protobuf.DurationR\bestError\x12\x16\n" +
	"\x06status\x18\x05 \x01(\tR\x06status\x12\x1a\n" +
	"\bconstant\x18\x06 \x01(\x03R\bconstant\x12\x1f\n" +
	"\vsync_status\x18\a \x01(\bR\n" +
	"syncStatus\x12\x14\n" +
	"\x05state\x18\b \x01(\tR\x05state\"_\n" +
	"\n" +
	"StatusSpec\x12\x16\n" +
	"\x06synced\x18\x01 \x01(\bR\x06synced\x12\x14\n" +
	"\x05epoch\x18\x02 \x01(\x03R\x05epoch\x12#\n" +
	"\rsync_disabled\x18\x03 \x01(\bR\fsyncDisabledBr\n" +
	"'dev.talos.api.resource.definitions.timeZGgithub.com/siderolabs/talos/pkg/machinery/api/resource/definitions/timeb\x06proto3"

var (
	file_resource_definitions_time_time_proto_rawDescOnce sync.Once
	file_resource_definitions_time_time_proto_rawDescData []byte
)

func file_resource_definitions_time_time_proto_rawDescGZIP() []byte {
	file_resource_definitions_time_time_proto_rawDescOnce.Do(func() {
		file_resource_definitions_time_time_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_resource_definitions_time_time_proto_rawDesc), len(file_resource_definitions_time_time_proto_rawDesc)))
	})
	return file_resource_definitions_time_time_proto_rawDescData
}

var file_resource_definitions_time_time_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_resource_definitions_time_time_proto_goTypes = []any{
	(*AdjtimeStatusSpec)(nil),   // 0: talos.resource.definitions.time.AdjtimeStatusSpec
	(*StatusSpec)(nil),          // 1: talos.resource.definitions.time.StatusSpec
	(*durationpb.Duration)(nil), // 2: google.protobuf.Duration
}
var file_resource_definitions_time_time_proto_depIdxs = []int32{
	2, // 0: talos.resource.definitions.time.AdjtimeStatusSpec.offset:type_name -> google.protobuf.Duration
	2, // 1: talos.resource.definitions.time.AdjtimeStatusSpec.max_error:type_name -> google.protobuf.Duration
	2, // 2: talos.resource.definitions.time.AdjtimeStatusSpec.est_error:type_name -> google.protobuf.Duration
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_resource_definitions_time_time_proto_init() }
func file_resource_definitions_time_time_proto_init() {
	if File_resource_definitions_time_time_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_resource_definitions_time_time_proto_rawDesc), len(file_resource_definitions_time_time_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_resource_definitions_time_time_proto_goTypes,
		DependencyIndexes: file_resource_definitions_time_time_proto_depIdxs,
		MessageInfos:      file_resource_definitions_time_time_proto_msgTypes,
	}.Build()
	File_resource_definitions_time_time_proto = out.File
	file_resource_definitions_time_time_proto_goTypes = nil
	file_resource_definitions_time_time_proto_depIdxs = nil
}
