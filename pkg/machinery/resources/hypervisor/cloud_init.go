// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

const (
	// CloudInitSpecType is the desired NoCloud seed resource type.
	CloudInitSpecType = resource.Type("CloudInitSpecs.hypervisor.talos.dev")
	// CloudInitStatusType is the produced NoCloud ISO resource type.
	CloudInitStatusType = resource.Type("CloudInitStatuses.hypervisor.talos.dev")
)

// CloudInitSpec is a VM's opaque desired NoCloud seed, keyed by VM name.
type CloudInitSpec = typed.Resource[CloudInitSpecSpec, CloudInitSpecExtension]

// CloudInitSpecSpec carries opaque payload bytes (strings preserve verbatim content).
//
//gotagsrewrite:gen
//redactgen:gen
type CloudInitSpecSpec struct {
	Library       string `yaml:"library" protobuf:"1"`
	MetaData      string `yaml:"metaData,omitempty" protobuf:"2" redact:"replace"`
	UserData      string `yaml:"userData,omitempty" protobuf:"3" redact:"replace"`
	NetworkConfig string `yaml:"networkConfig,omitempty" protobuf:"4" redact:"replace"`
}

// InputDigest identifies the content and library without exposing seed contents.
func (s CloudInitSpecSpec) InputDigest() string {
	h := sha256.New()

	for _, item := range []string{s.Library, s.MetaData, s.UserData, s.NetworkConfig} {
		writeHashPart(h, item)
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func writeHashPart(h hash.Hash, item string) {
	// A length prefix prevents ambiguous tuples, including embedded NUL bytes.
	size := uint64(len(item))

	var buf [8]byte

	for i := range buf {
		buf[i] = byte(size >> (8 * i))
	}

	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(item))
}

// CloudInitStatusID identifies a VM's immutable seed incarnation.
func CloudInitStatusID(vm string, spec CloudInitSpecSpec) resource.ID {
	h := sha256.New()
	writeHashPart(h, vm)
	writeHashPart(h, spec.InputDigest())

	return vm + "@" + hex.EncodeToString(h.Sum(nil))
}

// NewCloudInitSpec constructs the desired seed resource.
func NewCloudInitSpec(namespace resource.Namespace, id resource.ID) *CloudInitSpec {
	return typed.NewResource[CloudInitSpecSpec, CloudInitSpecExtension](
		resource.NewMetadata(namespace, CloudInitSpecType, id, resource.VersionUndefined),
		CloudInitSpecSpec{},
	)
}

// CloudInitSpecExtension defines the sensitive seed resource.
type CloudInitSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider.
func (CloudInitSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CloudInitSpecType,
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

// CloudInitStatus tracks one immutable asset, including the library backing it was produced on.
// Its ID is CloudInitStatusID; it stays held while an actual domain consumes the asset.
type CloudInitStatus = typed.Resource[CloudInitStatusSpec, CloudInitStatusExtension]

// CloudInitStatusSpec never contains the raw guest seed.
//
//gotagsrewrite:gen
type CloudInitStatusSpec struct {
	VirtualMachine string `yaml:"virtualMachine" protobuf:"1"`
	Library        string `yaml:"library" protobuf:"2"`
	Name           string `yaml:"name" protobuf:"3"`

	// Path and VolumeID pin the backing identity and prevent stale readiness across remounts.
	Path               string `yaml:"path,omitempty" protobuf:"4"`
	VolumeID           string `yaml:"volumeID,omitempty" protobuf:"5"`
	Digest             string `yaml:"digest,omitempty" protobuf:"6"`
	SizeBytes          uint64 `yaml:"sizeBytes" protobuf:"7"`
	InputDigest        string `yaml:"inputDigest" protobuf:"8"`
	ObservedGeneration string `yaml:"observedGeneration,omitempty" protobuf:"9"`
	Ready              bool   `yaml:"ready" protobuf:"10"`
	Error              string `yaml:"error,omitempty" protobuf:"11"`
}

// NewCloudInitStatus constructs a produced seed resource.
func NewCloudInitStatus(namespace resource.Namespace, id resource.ID) *CloudInitStatus {
	return typed.NewResource[CloudInitStatusSpec, CloudInitStatusExtension](
		resource.NewMetadata(namespace, CloudInitStatusType, id, resource.VersionUndefined),
		CloudInitStatusSpec{},
	)
}

// CloudInitStatusExtension defines a status containing no raw guest secrets.
type CloudInitStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider.
func (CloudInitStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             CloudInitStatusType,
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(CloudInitSpecType, &CloudInitSpec{}); err != nil {
		panic(err)
	}

	if err := protobuf.RegisterDynamic(CloudInitStatusType, &CloudInitStatus{}); err != nil {
		panic(err)
	}
}
