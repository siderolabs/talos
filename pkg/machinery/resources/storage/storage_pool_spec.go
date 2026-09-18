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

// StoragePoolSpecType is the type of StoragePoolSpec resource.
const StoragePoolSpecType = resource.Type("StoragePoolSpecs.storage.talos.dev")

// StoragePoolSpec is the desired state of a directory storage pool, keyed by pool name.
type StoragePoolSpec = typed.Resource[StoragePoolSpecSpec, StoragePoolSpecExtension]

// StoragePoolSpecSpec identifies the backing volume; the resource ID identifies the pool.
//
//gotagsrewrite:gen
type StoragePoolSpecSpec struct {
	VolumeID string `yaml:"volumeID" protobuf:"1"`
}

// NewStoragePoolSpec initializes a StoragePoolSpec resource.
func NewStoragePoolSpec(namespace resource.Namespace, id resource.ID) *StoragePoolSpec {
	return typed.NewResource[StoragePoolSpecSpec, StoragePoolSpecExtension](
		resource.NewMetadata(namespace, StoragePoolSpecType, id, resource.VersionUndefined),
		StoragePoolSpecSpec{},
	)
}

// StoragePoolSpecExtension is auxiliary resource data for StoragePoolSpec.
type StoragePoolSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider.
func (StoragePoolSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StoragePoolSpecType,
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic(StoragePoolSpecType, &StoragePoolSpec{}); err != nil {
		panic(err)
	}
}
