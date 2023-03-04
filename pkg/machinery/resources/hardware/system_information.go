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

// SystemInformationType is type of SystemInformation resource.
const SystemInformationType = resource.Type("SystemInformations.hardware.talos.dev")

// SystemInformationID is the ID of the SystemInformation resource.
const SystemInformationID = resource.ID("systeminformation")

// SystemInformation resource holds node SystemInformation information.
type SystemInformation = typed.Resource[SystemInformationSpec, SystemInformationExtension]

// SystemInformationSpec represents the system information obtained from smbios.
//
//gotagsrewrite:gen
type SystemInformationSpec struct {
	Manufacturer string `yaml:"manufacturer,omitempty" protobuf:"1"`
	ProductName  string `yaml:"productName,omitempty" protobuf:"2"`
	Version      string `yaml:"version,omitempty" protobuf:"3"`
	SerialNumber string `yaml:"serialnumber,omitempty" protobuf:"4"`
	UUID         string `yaml:"uuid,omitempty" protobuf:"5"`
	WakeUpType   string `yaml:"wakeUpType,omitempty" protobuf:"6"`
	SKUNumber    string `yaml:"skuNumber,omitempty" protobuf:"7"`
}

// NewSystemInformation initializes a SystemInformationInfo resource.
func NewSystemInformation(id string) *SystemInformation {
	return typed.NewResource[SystemInformationSpec, SystemInformationExtension](
		resource.NewMetadata(NamespaceName, SystemInformationType, id, resource.VersionUndefined),
		SystemInformationSpec{},
	)
}

// SystemInformationExtension provides auxiliary methods for SystemInformation.
type SystemInformationExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (SystemInformationExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: SystemInformationType,
		Aliases: []resource.Type{
			"systeminformation",
		},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Manufacturer",
				JSONPath: `{.manufacturer}`,
			},
			{
				Name:     "ProductName",
				JSONPath: `{.productName}`,
			},

			{
				Name:     "Version",
				JSONPath: `{.version}`,
			},

			{
				Name:     "SerialNumber",
				JSONPath: `{.serialnumber}`,
			},

			{
				Name:     "UUID",
				JSONPath: `{.uuid}`,
			},

			{
				Name:     "WakeUpType",
				JSONPath: `{.wakeUpType}`,
			},

			{
				Name:     "SKUNumber",
				JSONPath: `{.skuNumber}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[SystemInformationSpec](SystemInformationType, &SystemInformation{})
	if err != nil {
		panic(err)
	}
}
