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

// PCIDeviceType is type of PCIDevice resource.
const PCIDeviceType = resource.Type("PCIDevices.hardware.talos.dev")

// PCIDevice resource holds node PCIDevice information.
type PCIDevice = typed.Resource[PCIDeviceSpec, PCIDeviceExtension]

// PCIDeviceSpec represents a single processor.
//
//gotagsrewrite:gen
type PCIDeviceSpec struct {
	Class    string `yaml:"class,omitempty" protobuf:"1"`
	Subclass string `yaml:"subclass,omitempty" protobuf:"2"`
	Vendor   string `yaml:"vendor,omitempty" protobuf:"3"`
	Product  string `yaml:"product,omitempty" protobuf:"4"`

	ClassID    string `yaml:"class_id" protobuf:"5"`
	SubclassID string `yaml:"subclass_id" protobuf:"6"`
	VendorID   string `yaml:"vendor_id" protobuf:"7"`
	ProductID  string `yaml:"product_id" protobuf:"8"`
	Driver     string `yaml:"driver,omitempty" protobuf:"9"`
}

// NewPCIDeviceInfo initializes a PCIDeviceInfo resource.
func NewPCIDeviceInfo(id string) *PCIDevice {
	return typed.NewResource[PCIDeviceSpec, PCIDeviceExtension](
		resource.NewMetadata(NamespaceName, PCIDeviceType, id, resource.VersionUndefined),
		PCIDeviceSpec{},
	)
}

// PCIDeviceExtension provides auxiliary methods for PCIDevice info.
type PCIDeviceExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (PCIDeviceExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: PCIDeviceType,
		Aliases: []resource.Type{
			"devices",
			"device",
		},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Class",
				JSONPath: `{.class}`,
			},
			{
				Name:     "Subclass",
				JSONPath: `{.subclass}`,
			},
			{
				Name:     "Vendor",
				JSONPath: `{.vendor}`,
			},
			{
				Name:     "Product",
				JSONPath: `{.product}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(PCIDeviceType, &PCIDevice{})
	if err != nil {
		panic(err)
	}
}
