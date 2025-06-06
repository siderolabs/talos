// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v6.31.1
// source: cluster/cluster.proto

package cluster

import (
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"

	common "github.com/siderolabs/talos/pkg/machinery/api/common"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type HealthCheckRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	WaitTimeout   *durationpb.Duration   `protobuf:"bytes,1,opt,name=wait_timeout,json=waitTimeout,proto3" json:"wait_timeout,omitempty"`
	ClusterInfo   *ClusterInfo           `protobuf:"bytes,2,opt,name=cluster_info,json=clusterInfo,proto3" json:"cluster_info,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HealthCheckRequest) Reset() {
	*x = HealthCheckRequest{}
	mi := &file_cluster_cluster_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HealthCheckRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HealthCheckRequest) ProtoMessage() {}

func (x *HealthCheckRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cluster_cluster_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HealthCheckRequest.ProtoReflect.Descriptor instead.
func (*HealthCheckRequest) Descriptor() ([]byte, []int) {
	return file_cluster_cluster_proto_rawDescGZIP(), []int{0}
}

func (x *HealthCheckRequest) GetWaitTimeout() *durationpb.Duration {
	if x != nil {
		return x.WaitTimeout
	}
	return nil
}

func (x *HealthCheckRequest) GetClusterInfo() *ClusterInfo {
	if x != nil {
		return x.ClusterInfo
	}
	return nil
}

type ClusterInfo struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	ControlPlaneNodes []string               `protobuf:"bytes,1,rep,name=control_plane_nodes,json=controlPlaneNodes,proto3" json:"control_plane_nodes,omitempty"`
	WorkerNodes       []string               `protobuf:"bytes,2,rep,name=worker_nodes,json=workerNodes,proto3" json:"worker_nodes,omitempty"`
	ForceEndpoint     string                 `protobuf:"bytes,3,opt,name=force_endpoint,json=forceEndpoint,proto3" json:"force_endpoint,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *ClusterInfo) Reset() {
	*x = ClusterInfo{}
	mi := &file_cluster_cluster_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ClusterInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterInfo) ProtoMessage() {}

func (x *ClusterInfo) ProtoReflect() protoreflect.Message {
	mi := &file_cluster_cluster_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClusterInfo.ProtoReflect.Descriptor instead.
func (*ClusterInfo) Descriptor() ([]byte, []int) {
	return file_cluster_cluster_proto_rawDescGZIP(), []int{1}
}

func (x *ClusterInfo) GetControlPlaneNodes() []string {
	if x != nil {
		return x.ControlPlaneNodes
	}
	return nil
}

func (x *ClusterInfo) GetWorkerNodes() []string {
	if x != nil {
		return x.WorkerNodes
	}
	return nil
}

func (x *ClusterInfo) GetForceEndpoint() string {
	if x != nil {
		return x.ForceEndpoint
	}
	return ""
}

type HealthCheckProgress struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Metadata      *common.Metadata       `protobuf:"bytes,1,opt,name=metadata,proto3" json:"metadata,omitempty"`
	Message       string                 `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *HealthCheckProgress) Reset() {
	*x = HealthCheckProgress{}
	mi := &file_cluster_cluster_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *HealthCheckProgress) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HealthCheckProgress) ProtoMessage() {}

func (x *HealthCheckProgress) ProtoReflect() protoreflect.Message {
	mi := &file_cluster_cluster_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HealthCheckProgress.ProtoReflect.Descriptor instead.
func (*HealthCheckProgress) Descriptor() ([]byte, []int) {
	return file_cluster_cluster_proto_rawDescGZIP(), []int{2}
}

func (x *HealthCheckProgress) GetMetadata() *common.Metadata {
	if x != nil {
		return x.Metadata
	}
	return nil
}

func (x *HealthCheckProgress) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

var File_cluster_cluster_proto protoreflect.FileDescriptor

const file_cluster_cluster_proto_rawDesc = "" +
	"\n" +
	"\x15cluster/cluster.proto\x12\acluster\x1a\x13common/common.proto\x1a\x1egoogle/protobuf/duration.proto\"\x8b\x01\n" +
	"\x12HealthCheckRequest\x12<\n" +
	"\fwait_timeout\x18\x01 \x01(\v2\x19.google.protobuf.DurationR\vwaitTimeout\x127\n" +
	"\fcluster_info\x18\x02 \x01(\v2\x14.cluster.ClusterInfoR\vclusterInfo\"\x87\x01\n" +
	"\vClusterInfo\x12.\n" +
	"\x13control_plane_nodes\x18\x01 \x03(\tR\x11controlPlaneNodes\x12!\n" +
	"\fworker_nodes\x18\x02 \x03(\tR\vworkerNodes\x12%\n" +
	"\x0eforce_endpoint\x18\x03 \x01(\tR\rforceEndpoint\"]\n" +
	"\x13HealthCheckProgress\x12,\n" +
	"\bmetadata\x18\x01 \x01(\v2\x10.common.MetadataR\bmetadata\x12\x18\n" +
	"\amessage\x18\x02 \x01(\tR\amessage2\\\n" +
	"\x0eClusterService\x12J\n" +
	"\vHealthCheck\x12\x1b.cluster.HealthCheckRequest\x1a\x1c.cluster.HealthCheckProgress0\x01BN\n" +
	"\x15dev.talos.api.clusterZ5github.com/siderolabs/talos/pkg/machinery/api/clusterb\x06proto3"

var (
	file_cluster_cluster_proto_rawDescOnce sync.Once
	file_cluster_cluster_proto_rawDescData []byte
)

func file_cluster_cluster_proto_rawDescGZIP() []byte {
	file_cluster_cluster_proto_rawDescOnce.Do(func() {
		file_cluster_cluster_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_cluster_cluster_proto_rawDesc), len(file_cluster_cluster_proto_rawDesc)))
	})
	return file_cluster_cluster_proto_rawDescData
}

var file_cluster_cluster_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_cluster_cluster_proto_goTypes = []any{
	(*HealthCheckRequest)(nil),  // 0: cluster.HealthCheckRequest
	(*ClusterInfo)(nil),         // 1: cluster.ClusterInfo
	(*HealthCheckProgress)(nil), // 2: cluster.HealthCheckProgress
	(*durationpb.Duration)(nil), // 3: google.protobuf.Duration
	(*common.Metadata)(nil),     // 4: common.Metadata
}
var file_cluster_cluster_proto_depIdxs = []int32{
	3, // 0: cluster.HealthCheckRequest.wait_timeout:type_name -> google.protobuf.Duration
	1, // 1: cluster.HealthCheckRequest.cluster_info:type_name -> cluster.ClusterInfo
	4, // 2: cluster.HealthCheckProgress.metadata:type_name -> common.Metadata
	0, // 3: cluster.ClusterService.HealthCheck:input_type -> cluster.HealthCheckRequest
	2, // 4: cluster.ClusterService.HealthCheck:output_type -> cluster.HealthCheckProgress
	4, // [4:5] is the sub-list for method output_type
	3, // [3:4] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_cluster_cluster_proto_init() }
func file_cluster_cluster_proto_init() {
	if File_cluster_cluster_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_cluster_cluster_proto_rawDesc), len(file_cluster_cluster_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_cluster_cluster_proto_goTypes,
		DependencyIndexes: file_cluster_cluster_proto_depIdxs,
		MessageInfos:      file_cluster_cluster_proto_msgTypes,
	}.Build()
	File_cluster_cluster_proto = out.File
	file_cluster_cluster_proto_goTypes = nil
	file_cluster_cluster_proto_depIdxs = nil
}
