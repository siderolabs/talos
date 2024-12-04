// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// PCIRebindStatusType is the type of the PCIRebindStatus resource.
const PCIRebindStatusType = resource.Type("PCIRebindStatuses.runtime.talos.dev")

// PCIRebindStatus resource holds status of rebinded drivers.
type PCIRebindStatus = typed.Resource[PCIRebindStatusSpec, PCIRebindStatusExtension]

// PCIRebindStatusSpec describes status of rebinded drivers.
//
//gotagsrewrite:gen
type PCIRebindStatusSpec struct {
	Name           string `yaml:"name" protobuf:"1"`
	VendorDeviceID string `yaml:"vendorDeviceID" protobuf:"2"`
	HostDriver     string `yaml:"hostDriver" protobuf:"3"`
	TargetDriver   string `yaml:"targetDriver" protobuf:"4"`
}

// NewPCIRebindStatus initializes a PCIRebindStatus resource.
func NewPCIRebindStatus(id resource.ID) *PCIRebindStatus {
	return typed.NewResource[PCIRebindStatusSpec, PCIRebindStatusExtension](
		resource.NewMetadata(NamespaceName, PCIRebindStatusType, id, resource.VersionUndefined),
		PCIRebindStatusSpec{},
	)
}

// PCIRebindStatusExtension is auxiliary resource data for PCIRebindStatus.
type PCIRebindStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (PCIRebindStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PCIRebindStatusType,
		Aliases:          []resource.Type{"pcirebinds"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Name",
				JSONPath: `{.name}`,
			},
			{
				Name:     "VendorDeviceID",
				JSONPath: `{.vendorDeviceID}`,
			},
			{
				Name:     "HostDriver",
				JSONPath: `{.hostDriver}`,
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

	err := protobuf.RegisterDynamic[PCIRebindStatusSpec](PCIRebindStatusType, &PCIRebindStatus{})
	if err != nil {
		panic(err)
	}
}
