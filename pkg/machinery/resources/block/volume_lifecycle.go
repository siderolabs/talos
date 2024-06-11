// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// VolumeLifecycleType is type of VolumeLifecycle resource.
const VolumeLifecycleType = resource.Type("VolumeLifecycles.block.talos.dev")

// VolumeLifecycleID is the singleton ID of the resource.
const VolumeLifecycleID = resource.ID("volumes")

// VolumeLifecycle resource exists to signal that the volumes are to be closed.
//
// Volume manager controller puts a finalizer on this resource, and when
// this resource is being torn down, it will close all the volumes, and release the finalizer.
type VolumeLifecycle = typed.Resource[VolumeLifecycleSpec, VolumeLifecycleExtension]

// VolumeLifecycleSpec is empty.
type VolumeLifecycleSpec struct{}

// NewVolumeLifecycle initializes an empty VolumeLifecycle resource.
func NewVolumeLifecycle(namespace resource.Namespace, id resource.ID) *VolumeLifecycle {
	return typed.NewResource[VolumeLifecycleSpec, VolumeLifecycleExtension](
		resource.NewMetadata(namespace, VolumeLifecycleType, id, resource.VersionUndefined),
		VolumeLifecycleSpec{},
	)
}

// VolumeLifecycleExtension provides auxiliary methods for VolumeLifecycle.
type VolumeLifecycleExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (VolumeLifecycleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeLifecycleType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[VolumeLifecycleSpec](VolumeLifecycleType, &VolumeLifecycle{})
	if err != nil {
		panic(err)
	}
}
