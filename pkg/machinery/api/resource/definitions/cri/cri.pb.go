// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.5
// 	protoc        v5.29.3
// source: resource/definitions/cri/cri.proto

package cri

import (
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	structpb "google.golang.org/protobuf/types/known/structpb"

	common "github.com/siderolabs/talos/pkg/machinery/api/common"
	enums "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/enums"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// ImageCacheConfigSpec represents the ImageCacheConfig.
type ImageCacheConfigSpec struct {
	state         protoimpl.MessageState        `protogen:"open.v1"`
	Status        enums.CriImageCacheStatus     `protobuf:"varint,1,opt,name=status,proto3,enum=talos.resource.definitions.enums.CriImageCacheStatus" json:"status,omitempty"`
	Roots         []string                      `protobuf:"bytes,2,rep,name=roots,proto3" json:"roots,omitempty"`
	CopyStatus    enums.CriImageCacheCopyStatus `protobuf:"varint,3,opt,name=copy_status,json=copyStatus,proto3,enum=talos.resource.definitions.enums.CriImageCacheCopyStatus" json:"copy_status,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ImageCacheConfigSpec) Reset() {
	*x = ImageCacheConfigSpec{}
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ImageCacheConfigSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ImageCacheConfigSpec) ProtoMessage() {}

func (x *ImageCacheConfigSpec) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ImageCacheConfigSpec.ProtoReflect.Descriptor instead.
func (*ImageCacheConfigSpec) Descriptor() ([]byte, []int) {
	return file_resource_definitions_cri_cri_proto_rawDescGZIP(), []int{0}
}

func (x *ImageCacheConfigSpec) GetStatus() enums.CriImageCacheStatus {
	if x != nil {
		return x.Status
	}
	return enums.CriImageCacheStatus(0)
}

func (x *ImageCacheConfigSpec) GetRoots() []string {
	if x != nil {
		return x.Roots
	}
	return nil
}

func (x *ImageCacheConfigSpec) GetCopyStatus() enums.CriImageCacheCopyStatus {
	if x != nil {
		return x.CopyStatus
	}
	return enums.CriImageCacheCopyStatus(0)
}

// RegistriesConfigSpec describes status of rendered secrets.
type RegistriesConfigSpec struct {
	state           protoimpl.MessageState           `protogen:"open.v1"`
	RegistryMirrors map[string]*RegistryMirrorConfig `protobuf:"bytes,1,rep,name=registry_mirrors,json=registryMirrors,proto3" json:"registry_mirrors,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	RegistryConfig  map[string]*RegistryConfig       `protobuf:"bytes,2,rep,name=registry_config,json=registryConfig,proto3" json:"registry_config,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *RegistriesConfigSpec) Reset() {
	*x = RegistriesConfigSpec{}
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RegistriesConfigSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegistriesConfigSpec) ProtoMessage() {}

func (x *RegistriesConfigSpec) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RegistriesConfigSpec.ProtoReflect.Descriptor instead.
func (*RegistriesConfigSpec) Descriptor() ([]byte, []int) {
	return file_resource_definitions_cri_cri_proto_rawDescGZIP(), []int{1}
}

func (x *RegistriesConfigSpec) GetRegistryMirrors() map[string]*RegistryMirrorConfig {
	if x != nil {
		return x.RegistryMirrors
	}
	return nil
}

func (x *RegistriesConfigSpec) GetRegistryConfig() map[string]*RegistryConfig {
	if x != nil {
		return x.RegistryConfig
	}
	return nil
}

// RegistryAuthConfig specifies authentication configuration for a registry.
type RegistryAuthConfig struct {
	state                 protoimpl.MessageState `protogen:"open.v1"`
	RegistryUsername      string                 `protobuf:"bytes,1,opt,name=registry_username,json=registryUsername,proto3" json:"registry_username,omitempty"`
	RegistryPassword      string                 `protobuf:"bytes,2,opt,name=registry_password,json=registryPassword,proto3" json:"registry_password,omitempty"`
	RegistryAuth          string                 `protobuf:"bytes,3,opt,name=registry_auth,json=registryAuth,proto3" json:"registry_auth,omitempty"`
	RegistryIdentityToken string                 `protobuf:"bytes,4,opt,name=registry_identity_token,json=registryIdentityToken,proto3" json:"registry_identity_token,omitempty"`
	unknownFields         protoimpl.UnknownFields
	sizeCache             protoimpl.SizeCache
}

func (x *RegistryAuthConfig) Reset() {
	*x = RegistryAuthConfig{}
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RegistryAuthConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegistryAuthConfig) ProtoMessage() {}

func (x *RegistryAuthConfig) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RegistryAuthConfig.ProtoReflect.Descriptor instead.
func (*RegistryAuthConfig) Descriptor() ([]byte, []int) {
	return file_resource_definitions_cri_cri_proto_rawDescGZIP(), []int{2}
}

func (x *RegistryAuthConfig) GetRegistryUsername() string {
	if x != nil {
		return x.RegistryUsername
	}
	return ""
}

func (x *RegistryAuthConfig) GetRegistryPassword() string {
	if x != nil {
		return x.RegistryPassword
	}
	return ""
}

func (x *RegistryAuthConfig) GetRegistryAuth() string {
	if x != nil {
		return x.RegistryAuth
	}
	return ""
}

func (x *RegistryAuthConfig) GetRegistryIdentityToken() string {
	if x != nil {
		return x.RegistryIdentityToken
	}
	return ""
}

// RegistryConfig specifies auth & TLS config per registry.
type RegistryConfig struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RegistryTls   *RegistryTLSConfig     `protobuf:"bytes,1,opt,name=registry_tls,json=registryTls,proto3" json:"registry_tls,omitempty"`
	RegistryAuth  *RegistryAuthConfig    `protobuf:"bytes,2,opt,name=registry_auth,json=registryAuth,proto3" json:"registry_auth,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RegistryConfig) Reset() {
	*x = RegistryConfig{}
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RegistryConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegistryConfig) ProtoMessage() {}

func (x *RegistryConfig) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RegistryConfig.ProtoReflect.Descriptor instead.
func (*RegistryConfig) Descriptor() ([]byte, []int) {
	return file_resource_definitions_cri_cri_proto_rawDescGZIP(), []int{3}
}

func (x *RegistryConfig) GetRegistryTls() *RegistryTLSConfig {
	if x != nil {
		return x.RegistryTls
	}
	return nil
}

func (x *RegistryConfig) GetRegistryAuth() *RegistryAuthConfig {
	if x != nil {
		return x.RegistryAuth
	}
	return nil
}

// RegistryMirrorConfig represents mirror configuration for a registry.
type RegistryMirrorConfig struct {
	state              protoimpl.MessageState `protogen:"open.v1"`
	MirrorEndpoints    []string               `protobuf:"bytes,1,rep,name=mirror_endpoints,json=mirrorEndpoints,proto3" json:"mirror_endpoints,omitempty"`
	MirrorOverridePath bool                   `protobuf:"varint,2,opt,name=mirror_override_path,json=mirrorOverridePath,proto3" json:"mirror_override_path,omitempty"`
	MirrorSkipFallback bool                   `protobuf:"varint,3,opt,name=mirror_skip_fallback,json=mirrorSkipFallback,proto3" json:"mirror_skip_fallback,omitempty"`
	unknownFields      protoimpl.UnknownFields
	sizeCache          protoimpl.SizeCache
}

func (x *RegistryMirrorConfig) Reset() {
	*x = RegistryMirrorConfig{}
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RegistryMirrorConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegistryMirrorConfig) ProtoMessage() {}

func (x *RegistryMirrorConfig) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RegistryMirrorConfig.ProtoReflect.Descriptor instead.
func (*RegistryMirrorConfig) Descriptor() ([]byte, []int) {
	return file_resource_definitions_cri_cri_proto_rawDescGZIP(), []int{4}
}

func (x *RegistryMirrorConfig) GetMirrorEndpoints() []string {
	if x != nil {
		return x.MirrorEndpoints
	}
	return nil
}

func (x *RegistryMirrorConfig) GetMirrorOverridePath() bool {
	if x != nil {
		return x.MirrorOverridePath
	}
	return false
}

func (x *RegistryMirrorConfig) GetMirrorSkipFallback() bool {
	if x != nil {
		return x.MirrorSkipFallback
	}
	return false
}

// RegistryTLSConfig specifies TLS config for HTTPS registries.
type RegistryTLSConfig struct {
	state                 protoimpl.MessageState              `protogen:"open.v1"`
	TlsClientIdentity     *common.PEMEncodedCertificateAndKey `protobuf:"bytes,1,opt,name=tls_client_identity,json=tlsClientIdentity,proto3" json:"tls_client_identity,omitempty"`
	Tlsca                 []byte                              `protobuf:"bytes,2,opt,name=tlsca,proto3" json:"tlsca,omitempty"`
	TlsInsecureSkipVerify bool                                `protobuf:"varint,3,opt,name=tls_insecure_skip_verify,json=tlsInsecureSkipVerify,proto3" json:"tls_insecure_skip_verify,omitempty"`
	unknownFields         protoimpl.UnknownFields
	sizeCache             protoimpl.SizeCache
}

func (x *RegistryTLSConfig) Reset() {
	*x = RegistryTLSConfig{}
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RegistryTLSConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RegistryTLSConfig) ProtoMessage() {}

func (x *RegistryTLSConfig) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RegistryTLSConfig.ProtoReflect.Descriptor instead.
func (*RegistryTLSConfig) Descriptor() ([]byte, []int) {
	return file_resource_definitions_cri_cri_proto_rawDescGZIP(), []int{5}
}

func (x *RegistryTLSConfig) GetTlsClientIdentity() *common.PEMEncodedCertificateAndKey {
	if x != nil {
		return x.TlsClientIdentity
	}
	return nil
}

func (x *RegistryTLSConfig) GetTlsca() []byte {
	if x != nil {
		return x.Tlsca
	}
	return nil
}

func (x *RegistryTLSConfig) GetTlsInsecureSkipVerify() bool {
	if x != nil {
		return x.TlsInsecureSkipVerify
	}
	return false
}

// SeccompProfileSpec represents the SeccompProfile.
type SeccompProfileSpec struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Value         *structpb.Struct       `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SeccompProfileSpec) Reset() {
	*x = SeccompProfileSpec{}
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SeccompProfileSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SeccompProfileSpec) ProtoMessage() {}

func (x *SeccompProfileSpec) ProtoReflect() protoreflect.Message {
	mi := &file_resource_definitions_cri_cri_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SeccompProfileSpec.ProtoReflect.Descriptor instead.
func (*SeccompProfileSpec) Descriptor() ([]byte, []int) {
	return file_resource_definitions_cri_cri_proto_rawDescGZIP(), []int{6}
}

func (x *SeccompProfileSpec) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *SeccompProfileSpec) GetValue() *structpb.Struct {
	if x != nil {
		return x.Value
	}
	return nil
}

var File_resource_definitions_cri_cri_proto protoreflect.FileDescriptor

var file_resource_definitions_cri_cri_proto_rawDesc = string([]byte{
	0x0a, 0x22, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2f, 0x64, 0x65, 0x66, 0x69, 0x6e,
	0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2f, 0x63, 0x72, 0x69, 0x2f, 0x63, 0x72, 0x69, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1e, 0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2e, 0x72, 0x65, 0x73, 0x6f,
	0x75, 0x72, 0x63, 0x65, 0x2e, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x2e, 0x63, 0x72, 0x69, 0x1a, 0x13, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2f, 0x63, 0x6f, 0x6d,
	0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x73, 0x74, 0x72, 0x75, 0x63,
	0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x26, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63,
	0x65, 0x2f, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2f, 0x65, 0x6e,
	0x75, 0x6d, 0x73, 0x2f, 0x65, 0x6e, 0x75, 0x6d, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0xd7, 0x01, 0x0a, 0x14, 0x49, 0x6d, 0x61, 0x67, 0x65, 0x43, 0x61, 0x63, 0x68, 0x65, 0x43, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x53, 0x70, 0x65, 0x63, 0x12, 0x4d, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x35, 0x2e, 0x74, 0x61, 0x6c, 0x6f, 0x73,
	0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x65, 0x6e, 0x75, 0x6d, 0x73, 0x2e, 0x43, 0x72, 0x69, 0x49,
	0x6d, 0x61, 0x67, 0x65, 0x43, 0x61, 0x63, 0x68, 0x65, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52,
	0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x72, 0x6f, 0x6f, 0x74, 0x73,
	0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x05, 0x72, 0x6f, 0x6f, 0x74, 0x73, 0x12, 0x5a, 0x0a,
	0x0b, 0x63, 0x6f, 0x70, 0x79, 0x5f, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x39, 0x2e, 0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x2e, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e,
	0x65, 0x6e, 0x75, 0x6d, 0x73, 0x2e, 0x43, 0x72, 0x69, 0x49, 0x6d, 0x61, 0x67, 0x65, 0x43, 0x61,
	0x63, 0x68, 0x65, 0x43, 0x6f, 0x70, 0x79, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52, 0x0a, 0x63,
	0x6f, 0x70, 0x79, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0xec, 0x03, 0x0a, 0x14, 0x52, 0x65,
	0x67, 0x69, 0x73, 0x74, 0x72, 0x69, 0x65, 0x73, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x53, 0x70,
	0x65, 0x63, 0x12, 0x74, 0x0a, 0x10, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x5f, 0x6d,
	0x69, 0x72, 0x72, 0x6f, 0x72, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x49, 0x2e, 0x74,
	0x61, 0x6c, 0x6f, 0x73, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x64, 0x65,
	0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x63, 0x72, 0x69, 0x2e, 0x52, 0x65,
	0x67, 0x69, 0x73, 0x74, 0x72, 0x69, 0x65, 0x73, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x53, 0x70,
	0x65, 0x63, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x4d, 0x69, 0x72, 0x72, 0x6f,
	0x72, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x0f, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72,
	0x79, 0x4d, 0x69, 0x72, 0x72, 0x6f, 0x72, 0x73, 0x12, 0x71, 0x0a, 0x0f, 0x72, 0x65, 0x67, 0x69,
	0x73, 0x74, 0x72, 0x79, 0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x02, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x48, 0x2e, 0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72,
	0x63, 0x65, 0x2e, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x63,
	0x72, 0x69, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x69, 0x65, 0x73, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x53, 0x70, 0x65, 0x63, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x0e, 0x72, 0x65, 0x67,
	0x69, 0x73, 0x74, 0x72, 0x79, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x1a, 0x78, 0x0a, 0x14, 0x52,
	0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x4d, 0x69, 0x72, 0x72, 0x6f, 0x72, 0x73, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x4a, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x34, 0x2e, 0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2e, 0x72, 0x65, 0x73,
	0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x2e, 0x63, 0x72, 0x69, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x4d, 0x69,
	0x72, 0x72, 0x6f, 0x72, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x3a, 0x02, 0x38, 0x01, 0x1a, 0x71, 0x0a, 0x13, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72,
	0x79, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03,
	0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x44,
	0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2e, 0x2e,
	0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x64,
	0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x63, 0x72, 0x69, 0x2e, 0x52,
	0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0xcb, 0x01, 0x0a, 0x12, 0x52, 0x65, 0x67,
	0x69, 0x73, 0x74, 0x72, 0x79, 0x41, 0x75, 0x74, 0x68, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12,
	0x2b, 0x0a, 0x11, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x5f, 0x75, 0x73, 0x65, 0x72,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x10, 0x72, 0x65, 0x67, 0x69,
	0x73, 0x74, 0x72, 0x79, 0x55, 0x73, 0x65, 0x72, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x2b, 0x0a, 0x11,
	0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x5f, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72,
	0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x10, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72,
	0x79, 0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x12, 0x23, 0x0a, 0x0d, 0x72, 0x65, 0x67,
	0x69, 0x73, 0x74, 0x72, 0x79, 0x5f, 0x61, 0x75, 0x74, 0x68, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x0c, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x41, 0x75, 0x74, 0x68, 0x12, 0x36,
	0x0a, 0x17, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x5f, 0x69, 0x64, 0x65, 0x6e, 0x74,
	0x69, 0x74, 0x79, 0x5f, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x15, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74,
	0x79, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0xbf, 0x01, 0x0a, 0x0e, 0x52, 0x65, 0x67, 0x69, 0x73,
	0x74, 0x72, 0x79, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x54, 0x0a, 0x0c, 0x72, 0x65, 0x67,
	0x69, 0x73, 0x74, 0x72, 0x79, 0x5f, 0x74, 0x6c, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x31, 0x2e, 0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65,
	0x2e, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x63, 0x72, 0x69,
	0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x54, 0x4c, 0x53, 0x43, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x52, 0x0b, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x54, 0x6c, 0x73, 0x12,
	0x57, 0x0a, 0x0d, 0x72, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x5f, 0x61, 0x75, 0x74, 0x68,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x32, 0x2e, 0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2e, 0x72,
	0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x2e, 0x63, 0x72, 0x69, 0x2e, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79,
	0x41, 0x75, 0x74, 0x68, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x0c, 0x72, 0x65, 0x67, 0x69,
	0x73, 0x74, 0x72, 0x79, 0x41, 0x75, 0x74, 0x68, 0x22, 0xa5, 0x01, 0x0a, 0x14, 0x52, 0x65, 0x67,
	0x69, 0x73, 0x74, 0x72, 0x79, 0x4d, 0x69, 0x72, 0x72, 0x6f, 0x72, 0x43, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x12, 0x29, 0x0a, 0x10, 0x6d, 0x69, 0x72, 0x72, 0x6f, 0x72, 0x5f, 0x65, 0x6e, 0x64, 0x70,
	0x6f, 0x69, 0x6e, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0f, 0x6d, 0x69, 0x72,
	0x72, 0x6f, 0x72, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x73, 0x12, 0x30, 0x0a, 0x14,
	0x6d, 0x69, 0x72, 0x72, 0x6f, 0x72, 0x5f, 0x6f, 0x76, 0x65, 0x72, 0x72, 0x69, 0x64, 0x65, 0x5f,
	0x70, 0x61, 0x74, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x12, 0x6d, 0x69, 0x72, 0x72,
	0x6f, 0x72, 0x4f, 0x76, 0x65, 0x72, 0x72, 0x69, 0x64, 0x65, 0x50, 0x61, 0x74, 0x68, 0x12, 0x30,
	0x0a, 0x14, 0x6d, 0x69, 0x72, 0x72, 0x6f, 0x72, 0x5f, 0x73, 0x6b, 0x69, 0x70, 0x5f, 0x66, 0x61,
	0x6c, 0x6c, 0x62, 0x61, 0x63, 0x6b, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x12, 0x6d, 0x69,
	0x72, 0x72, 0x6f, 0x72, 0x53, 0x6b, 0x69, 0x70, 0x46, 0x61, 0x6c, 0x6c, 0x62, 0x61, 0x63, 0x6b,
	0x22, 0xb7, 0x01, 0x0a, 0x11, 0x52, 0x65, 0x67, 0x69, 0x73, 0x74, 0x72, 0x79, 0x54, 0x4c, 0x53,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x53, 0x0a, 0x13, 0x74, 0x6c, 0x73, 0x5f, 0x63, 0x6c,
	0x69, 0x65, 0x6e, 0x74, 0x5f, 0x69, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x23, 0x2e, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x50, 0x45, 0x4d,
	0x45, 0x6e, 0x63, 0x6f, 0x64, 0x65, 0x64, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61,
	0x74, 0x65, 0x41, 0x6e, 0x64, 0x4b, 0x65, 0x79, 0x52, 0x11, 0x74, 0x6c, 0x73, 0x43, 0x6c, 0x69,
	0x65, 0x6e, 0x74, 0x49, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x74,
	0x6c, 0x73, 0x63, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x74, 0x6c, 0x73, 0x63,
	0x61, 0x12, 0x37, 0x0a, 0x18, 0x74, 0x6c, 0x73, 0x5f, 0x69, 0x6e, 0x73, 0x65, 0x63, 0x75, 0x72,
	0x65, 0x5f, 0x73, 0x6b, 0x69, 0x70, 0x5f, 0x76, 0x65, 0x72, 0x69, 0x66, 0x79, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x15, 0x74, 0x6c, 0x73, 0x49, 0x6e, 0x73, 0x65, 0x63, 0x75, 0x72, 0x65,
	0x53, 0x6b, 0x69, 0x70, 0x56, 0x65, 0x72, 0x69, 0x66, 0x79, 0x22, 0x57, 0x0a, 0x12, 0x53, 0x65,
	0x63, 0x63, 0x6f, 0x6d, 0x70, 0x50, 0x72, 0x6f, 0x66, 0x69, 0x6c, 0x65, 0x53, 0x70, 0x65, 0x63,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x12, 0x2d, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74, 0x52, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x42, 0x70, 0x0a, 0x26, 0x64, 0x65, 0x76, 0x2e, 0x74, 0x61, 0x6c, 0x6f, 0x73,
	0x2e, 0x61, 0x70, 0x69, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x64, 0x65,
	0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x63, 0x72, 0x69, 0x5a, 0x46, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x69, 0x64, 0x65, 0x72, 0x6f,
	0x6c, 0x61, 0x62, 0x73, 0x2f, 0x74, 0x61, 0x6c, 0x6f, 0x73, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x6d,
	0x61, 0x63, 0x68, 0x69, 0x6e, 0x65, 0x72, 0x79, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x72, 0x65, 0x73,
	0x6f, 0x75, 0x72, 0x63, 0x65, 0x2f, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x2f, 0x63, 0x72, 0x69, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
})

var (
	file_resource_definitions_cri_cri_proto_rawDescOnce sync.Once
	file_resource_definitions_cri_cri_proto_rawDescData []byte
)

func file_resource_definitions_cri_cri_proto_rawDescGZIP() []byte {
	file_resource_definitions_cri_cri_proto_rawDescOnce.Do(func() {
		file_resource_definitions_cri_cri_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_resource_definitions_cri_cri_proto_rawDesc), len(file_resource_definitions_cri_cri_proto_rawDesc)))
	})
	return file_resource_definitions_cri_cri_proto_rawDescData
}

var file_resource_definitions_cri_cri_proto_msgTypes = make([]protoimpl.MessageInfo, 9)
var file_resource_definitions_cri_cri_proto_goTypes = []any{
	(*ImageCacheConfigSpec)(nil),               // 0: talos.resource.definitions.cri.ImageCacheConfigSpec
	(*RegistriesConfigSpec)(nil),               // 1: talos.resource.definitions.cri.RegistriesConfigSpec
	(*RegistryAuthConfig)(nil),                 // 2: talos.resource.definitions.cri.RegistryAuthConfig
	(*RegistryConfig)(nil),                     // 3: talos.resource.definitions.cri.RegistryConfig
	(*RegistryMirrorConfig)(nil),               // 4: talos.resource.definitions.cri.RegistryMirrorConfig
	(*RegistryTLSConfig)(nil),                  // 5: talos.resource.definitions.cri.RegistryTLSConfig
	(*SeccompProfileSpec)(nil),                 // 6: talos.resource.definitions.cri.SeccompProfileSpec
	nil,                                        // 7: talos.resource.definitions.cri.RegistriesConfigSpec.RegistryMirrorsEntry
	nil,                                        // 8: talos.resource.definitions.cri.RegistriesConfigSpec.RegistryConfigEntry
	(enums.CriImageCacheStatus)(0),             // 9: talos.resource.definitions.enums.CriImageCacheStatus
	(enums.CriImageCacheCopyStatus)(0),         // 10: talos.resource.definitions.enums.CriImageCacheCopyStatus
	(*common.PEMEncodedCertificateAndKey)(nil), // 11: common.PEMEncodedCertificateAndKey
	(*structpb.Struct)(nil),                    // 12: google.protobuf.Struct
}
var file_resource_definitions_cri_cri_proto_depIdxs = []int32{
	9,  // 0: talos.resource.definitions.cri.ImageCacheConfigSpec.status:type_name -> talos.resource.definitions.enums.CriImageCacheStatus
	10, // 1: talos.resource.definitions.cri.ImageCacheConfigSpec.copy_status:type_name -> talos.resource.definitions.enums.CriImageCacheCopyStatus
	7,  // 2: talos.resource.definitions.cri.RegistriesConfigSpec.registry_mirrors:type_name -> talos.resource.definitions.cri.RegistriesConfigSpec.RegistryMirrorsEntry
	8,  // 3: talos.resource.definitions.cri.RegistriesConfigSpec.registry_config:type_name -> talos.resource.definitions.cri.RegistriesConfigSpec.RegistryConfigEntry
	5,  // 4: talos.resource.definitions.cri.RegistryConfig.registry_tls:type_name -> talos.resource.definitions.cri.RegistryTLSConfig
	2,  // 5: talos.resource.definitions.cri.RegistryConfig.registry_auth:type_name -> talos.resource.definitions.cri.RegistryAuthConfig
	11, // 6: talos.resource.definitions.cri.RegistryTLSConfig.tls_client_identity:type_name -> common.PEMEncodedCertificateAndKey
	12, // 7: talos.resource.definitions.cri.SeccompProfileSpec.value:type_name -> google.protobuf.Struct
	4,  // 8: talos.resource.definitions.cri.RegistriesConfigSpec.RegistryMirrorsEntry.value:type_name -> talos.resource.definitions.cri.RegistryMirrorConfig
	3,  // 9: talos.resource.definitions.cri.RegistriesConfigSpec.RegistryConfigEntry.value:type_name -> talos.resource.definitions.cri.RegistryConfig
	10, // [10:10] is the sub-list for method output_type
	10, // [10:10] is the sub-list for method input_type
	10, // [10:10] is the sub-list for extension type_name
	10, // [10:10] is the sub-list for extension extendee
	0,  // [0:10] is the sub-list for field type_name
}

func init() { file_resource_definitions_cri_cri_proto_init() }
func file_resource_definitions_cri_cri_proto_init() {
	if File_resource_definitions_cri_cri_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_resource_definitions_cri_cri_proto_rawDesc), len(file_resource_definitions_cri_cri_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   9,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_resource_definitions_cri_cri_proto_goTypes,
		DependencyIndexes: file_resource_definitions_cri_cri_proto_depIdxs,
		MessageInfos:      file_resource_definitions_cri_cri_proto_msgTypes,
	}.Build()
	File_resource_definitions_cri_cri_proto = out.File
	file_resource_definitions_cri_cri_proto_goTypes = nil
	file_resource_definitions_cri_cri_proto_depIdxs = nil
}
