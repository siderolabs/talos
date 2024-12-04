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

// PCIRebindConfigType is type of PCIRebindConfig resource.
const PCIRebindConfigType = resource.Type("PCIRebindConfigs.runtime.talos.dev")

// PCIRebindConfig resource holds PCI rebind configuration.
type PCIRebindConfig = typed.Resource[PCIRebindConfigSpec, PCIRebindConfigExtension]

// PCIRebindConfigSpec describes PCI rebind configuration.
//
//gotagsrewrite:gen
type PCIRebindConfigSpec struct {
	Name           string `yaml:"name" protobuf:"1"`
	VendorDeviceID string `yaml:"vendorDeviceID" protobuf:"2"`
	TargetDriver   string `yaml:"targetDriver" protobuf:"3"`
}

// PCIRebindConfigExtension is auxiliary resource data for PCIRebindConfig.
type PCIRebindConfigExtension struct{}

// NewPCIRebindConfig initializes a PCIRebindConfig resource.
func NewPCIRebindConfig(id resource.ID) *PCIRebindConfig {
	return typed.NewResource[PCIRebindConfigSpec, PCIRebindConfigExtension](
		resource.NewMetadata(NamespaceName, PCIRebindConfigType, id, resource.VersionUndefined),
		PCIRebindConfigSpec{},
	)
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (PCIRebindConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PCIRebindConfigType,
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
				Name:     "TargetDriver",
				JSONPath: `{.targetDriver}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[PCIRebindConfigSpec](PCIRebindConfigType, &PCIRebindConfig{})
	if err != nil {
		panic(err)
	}
}
