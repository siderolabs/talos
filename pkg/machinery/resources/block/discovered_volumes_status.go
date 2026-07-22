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

// DiscoveredVolumesStatusType is type of DiscoveredVolumesStatus resource.
const DiscoveredVolumesStatusType = resource.Type("DiscoveredVolumesStatuses.block.talos.dev")

// DiscoveredVolumesStatus resource holds the volume discovery status (overall).
type DiscoveredVolumesStatus = typed.Resource[DiscoveredVolumesStatusSpec, DiscoveredVolumesStatusExtension]

// DiscoveredVolumesStatusID the ID of DiscoveredVolumesStatus resource.
const DiscoveredVolumesStatusID = resource.ID("discovered-volumes-status")

// DiscoveredVolumesStatusSpec is the spec for discovered volumes status.
//
//gotagsrewrite:gen
type DiscoveredVolumesStatusSpec struct {
	// Volume discovery has been completed and the discovered volumes are ready to be used.
	Ready bool `yaml:"ready" protobuf:"1"`
}

// NewDiscoveredVolumesStatus initializes a DiscoveredVolumesStatus resource.
func NewDiscoveredVolumesStatus(namespace resource.Namespace, id resource.ID) *DiscoveredVolumesStatus {
	return typed.NewResource[DiscoveredVolumesStatusSpec, DiscoveredVolumesStatusExtension](
		resource.NewMetadata(namespace, DiscoveredVolumesStatusType, id, resource.VersionUndefined),
		DiscoveredVolumesStatusSpec{},
	)
}

// DiscoveredVolumesStatusExtension is auxiliary resource data for DiscoveredVolumesStatus.
type DiscoveredVolumesStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (DiscoveredVolumesStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DiscoveredVolumesStatusType,
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

	err := protobuf.RegisterDynamic[DiscoveredVolumesStatusSpec](DiscoveredVolumesStatusType, &DiscoveredVolumesStatus{})
	if err != nil {
		panic(err)
	}
}
