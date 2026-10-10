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

// StoragePoolVolumeStatusType is the type of StoragePoolVolumeStatus resource.
const StoragePoolVolumeStatusType = resource.Type("StoragePoolVolumeStatuses.storage.talos.dev")

// StoragePoolVolumeStatus reports what a volume in a storage pool actually is.
//
// A volume is never deleted: this resource going away means nothing is asking for the volume any
// more, not that its contents are gone. These statuses do not inventory retained orphan files;
// operators must inspect the pool itself when reclaiming space by hand.
//
// The ID is built by StoragePoolVolumeID, the same one its spec carries.
type StoragePoolVolumeStatus = typed.Resource[StoragePoolVolumeStatusSpec, StoragePoolVolumeStatusExtension]

// StoragePoolVolumeStatusSpec is the spec for StoragePoolVolumeStatus.
//
//gotagsrewrite:gen
type StoragePoolVolumeStatusSpec struct {
	// Pool is the name of the storage pool the volume lives in.
	Pool string `yaml:"pool" protobuf:"1"`
	// Name is the volume's name within that pool.
	Name string `yaml:"name" protobuf:"2"`
	// Path is the absolute host path of the volume's file.
	//
	// Only meaningful when Ready.
	Path string `yaml:"path,omitempty" protobuf:"3"`
	// Format is the volume's actual on-disk format, as libvirt reports it.
	//
	// Only meaningful when Ready.
	Format string `yaml:"format,omitempty" protobuf:"4"`
	// Capacity is the volume's actual logical size in bytes.
	//
	// It may exceed the capacity asked for: a volume is never shrunk.
	Capacity uint64 `yaml:"capacity,omitempty" protobuf:"5"`
	// PendingCapacity is a growth which has been asked for but not applied yet, in bytes.
	//
	// Zero when there is none. A volume a guest has open cannot be grown underneath it, so the
	// change waits for the guest to stop rather than being lost or forced.
	PendingCapacity uint64 `yaml:"pendingCapacity,omitempty" protobuf:"6"`
	// Phase reports whether the volume may be attached, independently of deferred growth or Error.
	Phase StoragePoolVolumePhase `yaml:"phase" protobuf:"7"`
	// Error describes why the volume is not ready, or -- when it is -- which part of what was asked
	// for was refused.
	Error string `yaml:"error,omitempty" protobuf:"8"`
}

// NewStoragePoolVolumeStatus initializes a StoragePoolVolumeStatus resource.
func NewStoragePoolVolumeStatus(namespace resource.Namespace, id resource.ID) *StoragePoolVolumeStatus {
	return typed.NewResource[StoragePoolVolumeStatusSpec, StoragePoolVolumeStatusExtension](
		resource.NewMetadata(namespace, StoragePoolVolumeStatusType, id, resource.VersionUndefined),
		StoragePoolVolumeStatusSpec{},
	)
}

// StoragePoolVolumeStatusExtension is auxiliary resource data for StoragePoolVolumeStatus.
type StoragePoolVolumeStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider.
func (StoragePoolVolumeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StoragePoolVolumeStatusType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Pool", JSONPath: "{.pool}"},
			{Name: "Name", JSONPath: "{.name}"}, //nolint:goconst
			{Name: "Capacity", JSONPath: "{.capacity}"},
			{Name: "Path", JSONPath: "{.path}"},
			{Name: "Phase", JSONPath: "{.phase}"},
			{Name: "Error", JSONPath: "{.error}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(StoragePoolVolumeStatusType, &StoragePoolVolumeStatus{}); err != nil {
		panic(err)
	}
}
