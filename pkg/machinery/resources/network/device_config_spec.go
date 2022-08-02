// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

// DeviceConfigSpecType is type of DeviceConfigSpec resource.
const DeviceConfigSpecType = resource.Type("DeviceConfigSpecs.net.talos.dev")

// DeviceConfigSpec resource holds network interface configs.
type DeviceConfigSpec = typed.Resource[DeviceConfigSpecSpec, DeviceConfigSpecRD]

// DeviceConfigSpecSpec contains the spec of a device config.
//
//gotagsrewrite:gen
type DeviceConfigSpecSpec struct {
	Device config.Device `protobuf:"1"`
}

// NewDeviceConfig creates new interface config.
func NewDeviceConfig(id resource.ID, device config.Device) *DeviceConfigSpec {
	return typed.NewResource[DeviceConfigSpecSpec, DeviceConfigSpecRD](
		resource.NewMetadata(NamespaceName, DeviceConfigSpecType, id, resource.VersionUndefined),
		DeviceConfigSpecSpec{Device: device},
	)
}

// DeepCopy generates a deep copy of DeviceConfigSpecSpec.
func (spec DeviceConfigSpecSpec) DeepCopy() DeviceConfigSpecSpec {
	cp := spec
	cp.Device = spec.Device.(*v1alpha1.Device).DeepCopy()

	return cp
}

// DeviceConfigSpecRD providers auxiliary methods for DeviceConfigSpec.
type DeviceConfigSpecRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (DeviceConfigSpecRD) ResourceDefinition(resource.Metadata, DeviceConfigSpecSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DeviceConfigSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}
