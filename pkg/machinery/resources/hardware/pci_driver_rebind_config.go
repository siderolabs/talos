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

// PCIDriverRebindConfigType is type of PCIDriverRebindConfig resource.
const PCIDriverRebindConfigType = resource.Type("PCIDriverRebindConfigs.runtime.talos.dev")

// PCIDriverRebindConfig resource holds PCI rebind configuration.
type PCIDriverRebindConfig = typed.Resource[PCIDriverRebindConfigSpec, PCIDriverRebindConfigExtension]

// PCIDriverRebindConfigSpec describes PCI rebind configuration.
//
//gotagsrewrite:gen
type PCIDriverRebindConfigSpec struct {
	PCIID        string `yaml:"pciID" protobuf:"1"`
	TargetDriver string `yaml:"targetDriver" protobuf:"2"`
}

// PCIDriverRebindConfigExtension is auxiliary resource data for PCIDriverRebindConfig.
type PCIDriverRebindConfigExtension struct{}

// NewPCIDriverRebindConfig initializes a PCIDriverRebindConfig resource.
func NewPCIDriverRebindConfig(id resource.ID) *PCIDriverRebindConfig {
	return typed.NewResource[PCIDriverRebindConfigSpec, PCIDriverRebindConfigExtension](
		resource.NewMetadata(NamespaceName, PCIDriverRebindConfigType, id, resource.VersionUndefined),
		PCIDriverRebindConfigSpec{},
	)
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (PCIDriverRebindConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PCIDriverRebindConfigType,
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

	err := protobuf.RegisterDynamic[PCIDriverRebindConfigSpec](PCIDriverRebindConfigType, &PCIDriverRebindConfig{})
	if err != nil {
		panic(err)
	}
}
