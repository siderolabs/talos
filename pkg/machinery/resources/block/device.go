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

// DeviceType is type of BlockDevice resource.
const DeviceType = resource.Type("BlockDevices.block.talos.dev")

// Device resource holds status of hardware devices (overall).
type Device = typed.Resource[DeviceSpec, DeviceExtension]

// DeviceSpec is the spec for devices status.
//
//gotagsrewrite:gen
type DeviceSpec struct {
	Type            string `yaml:"type" protobuf:"1"`
	Major           int    `yaml:"major" protobuf:"2"`
	Minor           int    `yaml:"minor" protobuf:"3"`
	PartitionName   string `yaml:"partitionName,omitempty" protobuf:"4"`
	PartitionNumber int    `yaml:"partitionNumber,omitempty" protobuf:"5"`
	DevicePath      string `yaml:"devicePath" protobuf:"7"`

	// Generation is bumped every time the device might have changed and might need to be re-probed.
	Generation int `yaml:"generation" protobuf:"6"`

	// Parent (if set) specifies the parent device ID.
	Parent string `yaml:"parent,omitempty" protobuf:"8"`

	// Secondaries (if set) specifies the secondary device IDs.
	//
	// E.g. for a LVM volume secondary is a list of blockdevices that the volume consists of.
	Secondaries []string `yaml:"secondaries,omitempty" protobuf:"9"`
}

// DeviceType constants.
const (
	DeviceTypeDisk      = "disk"
	DeviceTypePartition = "partition"
)

// NewDevice initializes a BlockDevice resource.
func NewDevice(namespace resource.Namespace, id resource.ID) *Device {
	return typed.NewResource[DeviceSpec, DeviceExtension](
		resource.NewMetadata(namespace, DeviceType, id, resource.VersionUndefined),
		DeviceSpec{},
	)
}

// DeviceExtension is auxiliary resource data for BlockDevice.
type DeviceExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (DeviceExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DeviceType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Type",
				JSONPath: `{.type}`,
			},
			{
				Name:     "PartitionName",
				JSONPath: `{.partitionName}`,
			},
			{
				Name:     "Generation",
				JSONPath: `{.generation}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(DeviceType, &Device{})
	if err != nil {
		panic(err)
	}
}
