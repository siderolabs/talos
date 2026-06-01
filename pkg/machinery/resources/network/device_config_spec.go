// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"go.yaml.in/yaml/v4"

	networkpb "github.com/siderolabs/talos/pkg/machinery/api/resource/network"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// DeviceConfigSpecType is type of DeviceConfigSpec resource.
const DeviceConfigSpecType = resource.Type("DeviceConfigSpecs.net.talos.dev")

// DeviceConfigSpec resource holds network interface configs.
type DeviceConfigSpec = typed.Resource[DeviceConfigSpecSpec, DeviceConfigSpecExtension]

// DeviceConfigSpecSpec contains the spec of a device config.
//
//gotagsrewrite:gen
type DeviceConfigSpecSpec struct {
	Device config.Device `protobuf:"1"`
}

// NewDeviceConfig creates new interface config.
func NewDeviceConfig(id resource.ID, device config.Device) *DeviceConfigSpec {
	return typed.NewResource[DeviceConfigSpecSpec, DeviceConfigSpecExtension](
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

// DeviceConfigSpecExtension providers auxiliary methods for DeviceConfigSpec.
type DeviceConfigSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (DeviceConfigSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DeviceConfigSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}

// MarshalProto implements ProtoMarshaler.
func (spec *DeviceConfigSpecSpec) MarshalProto() ([]byte, error) {
	yamlBytes, err := yaml.Marshal(spec.Device)
	if err != nil {
		return nil, err
	}

	protoSpec := networkpb.DeviceConfigSpecSpec{
		YamlMarshalled: yamlBytes,
	}

	return proto.Marshal(&protoSpec)
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (spec *DeviceConfigSpecSpec) UnmarshalProto(protoBytes []byte) error {
	protoSpec := networkpb.DeviceConfigSpecSpec{}

	if err := proto.Unmarshal(protoBytes, &protoSpec); err != nil {
		return err
	}

	var dev v1alpha1.Device

	if err := yaml.Unmarshal(protoSpec.YamlMarshalled, &dev); err != nil {
		return err
	}

	spec.Device = &dev

	return nil
}

func init() {
	if err := protobuf.RegisterResource(DeviceConfigSpecType, &DeviceConfigSpec{}); err != nil {
		panic(err)
	}
}
