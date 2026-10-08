// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// StoragePoolVolumeSpecType is the type of StoragePoolVolumeSpec resource.
const StoragePoolVolumeSpecType = resource.Type("StoragePoolVolumeSpecs.storage.talos.dev")

// StoragePoolVolumeSpec asks for a volume to exist in a storage pool.
//
// It is authored by whatever needs the volume -- today the hypervisor, for a blank virtual machine
// disk -- and acted on by the storage slice, which owns the connection to the storage daemon.
//
// The ID is built by StoragePoolVolumeID.
type StoragePoolVolumeSpec = typed.Resource[StoragePoolVolumeSpecSpec, StoragePoolVolumeSpecExtension]

// StoragePoolVolumeSpecSpec is the spec for StoragePoolVolumeSpec.
//
//gotagsrewrite:gen
type StoragePoolVolumeSpecSpec struct {
	// Pool is the name of the storage pool the volume lives in.
	Pool string `yaml:"pool" protobuf:"1"`
	// Name is the volume's name within that pool, which is its file name in the pool directory.
	Name string `yaml:"name" protobuf:"2"`
	// Capacity is the logical size in bytes the volume is asked to have.
	//
	// A volume is only ever grown towards it. One already larger is left alone: shrinking would
	// truncate a filesystem its owner, not Talos, laid out.
	Capacity uint64 `yaml:"capacity" protobuf:"3"`
	// Format is the on-disk format the volume is created with, as libvirt names it.
	//
	// It applies at creation only. A volume already in the pool under another format is refused
	// rather than rewritten.
	Format string `yaml:"format" protobuf:"4"`
	// BackingFile is an optional base image path used only when creating the volume.
	// An existing volume is not rebased when this changes.
	BackingFile string `yaml:"backingFile,omitempty" protobuf:"5"`
	// BackingFormat is the format of BackingFile, independent of the overlay Format.
	// If omitted, libvirt defaults the base to raw. It applies only at creation; existing volumes are not rebased.
	BackingFormat string `yaml:"backingFormat,omitempty" protobuf:"6"`
}

// StoragePoolVolumeID builds the resource ID of a volume in a pool.
//
// A pool name cannot contain a slash, and neither can a volume name, which is a single file name
// within the pool's directory.
func StoragePoolVolumeID(pool, name string) resource.ID {
	return pool + "/" + name
}

// NewStoragePoolVolumeSpec initializes a StoragePoolVolumeSpec resource.
func NewStoragePoolVolumeSpec(namespace resource.Namespace, id resource.ID) *StoragePoolVolumeSpec {
	return typed.NewResource[StoragePoolVolumeSpecSpec, StoragePoolVolumeSpecExtension](
		resource.NewMetadata(namespace, StoragePoolVolumeSpecType, id, resource.VersionUndefined),
		StoragePoolVolumeSpecSpec{},
	)
}

// StoragePoolVolumeSpecExtension is auxiliary resource data for StoragePoolVolumeSpec.
type StoragePoolVolumeSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider.
func (StoragePoolVolumeSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StoragePoolVolumeSpecType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Pool", JSONPath: "{.pool}"},
			{Name: "Name", JSONPath: "{.name}"}, //nolint:goconst
			{Name: "Capacity", JSONPath: "{.capacity}"},
			{Name: "Format", JSONPath: "{.format}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(StoragePoolVolumeSpecType, &StoragePoolVolumeSpec{}); err != nil {
		panic(err)
	}
}
