// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// BMCDeviceType is type of BMCDevice resource.
const BMCDeviceType = resource.Type("BMCDevices.hardware.talos.dev")

// BMCDevice resource holds information about a baseboard management controller (BMC)
// discovered on the host.
//
// A single resource is produced per local IPMI device, the resource ID being the
// name of the device node (e.g. `ipmi0`).
type BMCDevice = typed.Resource[BMCDeviceSpec, BMCDeviceExtension]

// BMCDeviceSpec describes a BMC as reported over the local IPMI interface.
//
// The identity fields come from Get Device ID, the network configuration from
// Get LAN Configuration Parameters. Network configuration is best-effort: a BMC
// with no LAN channel configured (or one which doesn't implement the commands)
// yields the identity fields only.
//
//gotagsrewrite:gen
type BMCDeviceSpec struct {
	// ManufacturerID is the IANA enterprise number of the BMC vendor (e.g. 674 for Dell).
	ManufacturerID uint32 `yaml:"manufacturerID,omitempty" protobuf:"1"`
	// Manufacturer is the vendor name resolved from ManufacturerID, empty if the vendor is not known.
	Manufacturer string `yaml:"manufacturer,omitempty" protobuf:"2"`
	// ProductID is the vendor-specific product identifier of the BMC.
	ProductID uint32 `yaml:"productID,omitempty" protobuf:"3"`
	// FirmwareVersion is the BMC firmware revision, e.g. `7.10`.
	FirmwareVersion string `yaml:"firmwareVersion,omitempty" protobuf:"4"`
	// IPMIVersion is the IPMI specification version supported by the BMC, e.g. `2.0`.
	IPMIVersion string `yaml:"ipmiVersion,omitempty" protobuf:"5"`
	// Channel is the IPMI LAN channel the network configuration was read from.
	Channel uint32 `yaml:"channel,omitempty" protobuf:"6"`
	// Address is the BMC IP address with its subnet mask.
	Address netip.Prefix `yaml:"address,omitempty" protobuf:"7"`
	// Gateway is the BMC default gateway.
	Gateway netip.Addr `yaml:"gateway,omitempty" protobuf:"8"`
	// HardwareAddr is the MAC address of the BMC LAN interface.
	HardwareAddr nethelpers.HardwareAddr `yaml:"hardwareAddr,omitempty" protobuf:"9"`
}

// NewBMCDevice initializes a BMCDevice resource.
func NewBMCDevice(id string) *BMCDevice {
	return typed.NewResource[BMCDeviceSpec, BMCDeviceExtension](
		resource.NewMetadata(NamespaceName, BMCDeviceType, id, resource.VersionUndefined),
		BMCDeviceSpec{},
	)
}

// BMCDeviceExtension provides auxiliary methods for BMCDevice info.
type BMCDeviceExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (BMCDeviceExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: BMCDeviceType,
		Aliases: []resource.Type{
			"bmc",
		},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Manufacturer",
				JSONPath: `{.manufacturer}`,
			},
			{
				Name:     "Firmware",
				JSONPath: `{.firmwareVersion}`,
			},
			{
				Name:     "Address",
				JSONPath: `{.address}`,
			},
			{
				Name:     "MAC",
				JSONPath: `{.hardwareAddr}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[BMCDeviceSpec](BMCDeviceType, &BMCDevice{})
	if err != nil {
		panic(err)
	}
}
