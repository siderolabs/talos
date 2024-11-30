// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// PCIDriverRebindStatusType is the type of the PCIDriverRebindStatus resource.
const PCIDriverRebindStatusType = resource.Type("PCIDriverRebindStatuses.runtime.talos.dev")

// PCIDriverRebindStatus resource holds status of rebinded drivers.
type PCIDriverRebindStatus = typed.Resource[PCIDriverRebindStatusSpec, PCIDriverRebindStatusExtension]

// PCIDriverRebindStatusSpec describes status of rebinded drivers.
//
//gotagsrewrite:gen
type PCIDriverRebindStatusSpec struct {
	PCIID        string `yaml:"pciID" protobuf:"1"`
	TargetDriver string `yaml:"targetDriver" protobuf:"2"`
}

// NewPCIDriverRebindStatus initializes a PCIDriverRebindStatus resource.
func NewPCIDriverRebindStatus(id resource.ID) *PCIDriverRebindStatus {
	return typed.NewResource[PCIDriverRebindStatusSpec, PCIDriverRebindStatusExtension](
		resource.NewMetadata(NamespaceName, PCIDriverRebindStatusType, id, resource.VersionUndefined),
		PCIDriverRebindStatusSpec{},
	)
}

// PCIDriverRebindStatusExtension is auxiliary resource data for PCIDriverRebindStatus.
type PCIDriverRebindStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (PCIDriverRebindStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PCIDriverRebindStatusType,
		Aliases:          []resource.Type{"pcidriverrebinds"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Name",
				JSONPath: `{.name}`,
			},
			{
				Name:     "PCI ID",
				JSONPath: `{.pciID}`,
			},
			{
				Name:     "TargetDriver",
				JSONPath: `{.targetDriver}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[PCIDriverRebindStatusSpec](PCIDriverRebindStatusType, &PCIDriverRebindStatus{})
	if err != nil {
		panic(err)
	}
}
