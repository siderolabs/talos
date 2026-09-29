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

// StoragePoolStatusType is the type of StoragePoolStatus resource.
const StoragePoolStatusType = resource.Type("StoragePoolStatuses.storage.talos.dev")

// StoragePoolStatus is the observed state of a directory storage pool, keyed by pool name.
type StoragePoolStatus = typed.Resource[StoragePoolStatusSpec, StoragePoolStatusExtension]

// StoragePoolStatusSpec reports the pool's backing volume, directory and readiness.
//
//gotagsrewrite:gen
type StoragePoolStatusSpec struct {
	VolumeID   string `yaml:"volumeID" protobuf:"1"`
	TargetPath string `yaml:"targetPath" protobuf:"2"`
	Ready      bool   `yaml:"ready" protobuf:"3"`
	Error      string `yaml:"error,omitempty" protobuf:"4"`
}

// NewStoragePoolStatus initializes a StoragePoolStatus resource.
func NewStoragePoolStatus(namespace resource.Namespace, id resource.ID) *StoragePoolStatus {
	return typed.NewResource[StoragePoolStatusSpec, StoragePoolStatusExtension](
		resource.NewMetadata(namespace, StoragePoolStatusType, id, resource.VersionUndefined),
		StoragePoolStatusSpec{},
	)
}

// StoragePoolStatusExtension is auxiliary resource data for StoragePoolStatus.
type StoragePoolStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider.
func (StoragePoolStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StoragePoolStatusType,
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{Name: "Volume", JSONPath: "{.volumeID}"},
			{Name: "Target", JSONPath: "{.targetPath}"},
			{Name: "Ready", JSONPath: "{.ready}"},
			{Name: "Error", JSONPath: "{.error}"},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(StoragePoolStatusType, &StoragePoolStatus{}); err != nil {
		panic(err)
	}
}
