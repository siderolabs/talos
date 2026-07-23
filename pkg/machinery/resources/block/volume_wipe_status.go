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

// VolumeWipeStatusType is type of VolumeWipeStatus resource.
const VolumeWipeStatusType = resource.Type("VolumeWipeStatuses.block.talos.dev")

// VolumeWipeStatus resource holds status of a volume wipe operation.
type VolumeWipeStatus = typed.Resource[VolumeWipeStatusSpec, VolumeWipeStatusExtension]

const VolumeWipeID = resource.ID("volume-wipe")

// VolumeWipeStatusSpec is the spec for VolumeWipeStatus resource.
//
//gotagsrewrite:gen
type VolumeWipeStatusSpec struct {
	// Ready indicates whether the volume wiping has completed successfully.
	Ready bool `yaml:"ready" protobuf:"1"`
}

// NewVolumeWipeStatus initializes a VolumeWipeStatus resource.
func NewVolumeWipeStatus(namespace resource.Namespace, id resource.ID) *VolumeWipeStatus {
	return typed.NewResource[VolumeWipeStatusSpec, VolumeWipeStatusExtension](
		resource.NewMetadata(namespace, VolumeWipeStatusType, id, resource.VersionUndefined),
		VolumeWipeStatusSpec{},
	)
}

// VolumeWipeStatusExtension is auxiliary resource data for VolumeWipeStatus.
type VolumeWipeStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VolumeWipeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeWipeStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: `{.ready}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[VolumeWipeStatusSpec](VolumeWipeStatusType, &VolumeWipeStatus{})
	if err != nil {
		panic(err)
	}
}
