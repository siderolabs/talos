// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// SystemInformationType is type of SystemInformation resource.
const SystemInformationType = resource.Type("SystemInformation.hardware.talos.dev")

// SystemInformation resource holds node SystemInformation information.
type SystemInformation = typed.Resource[SystemInformationSpec, SystemInformationRD]

// SystemInformationSpec represents the system information obtained from smbios.
type SystemInformationSpec struct {
	Manufacturer string `yaml:"manufacturer",omitempty`
	ProductName  string `yaml:"productName",omitempty`
	Version      string `yaml:"version",omitempty`
	SerialNumber string `yaml:"serialnumber",omitempty`
	UUID         string `yaml:"uuid",omitempty`
	WakeUpType   string `yaml:"wakeUpType",omitempty`
	SKUNumber    string `yaml:"skuNumber",omitempty`
}

// NewSystemInformation initializes a SystemInformationInfo resource.
func NewSystemInformation(id string) *SystemInformation {
	return typed.NewResource[SystemInformationSpec, SystemInformationRD](
		resource.NewMetadata(NamespaceName, SystemInformationType, id, resource.VersionUndefined),
		SystemInformationSpec{},
	)
}

// SystemInformationRD provides auxiliary methods for SystemInformation.
type SystemInformationRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (c SystemInformationRD) ResourceDefinition(resource.Metadata, SystemInformationSpec) meta.ResourceDefinitionSpec {
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
