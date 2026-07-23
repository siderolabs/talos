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

// SMARTStatusType is type of SMARTStatus resource.
const SMARTStatusType = resource.Type("SMARTStatuses.block.talos.dev")

// SMARTStatus resource holds SMART health information for a disk.
//
// It is keyed by the same ID as the Disk resource (the device name, e.g. "sda").
type SMARTStatus = typed.Resource[SMARTStatusSpec, SMARTStatusExtension]

// SMARTStatusSpec is the spec for SMARTStatus resource.
//
//gotagsrewrite:gen
type SMARTStatusSpec struct {
	// DevPath is the path to the block device, e.g. /dev/sda.
	DevPath string `yaml:"dev_path" protobuf:"1"`
	// DeviceType is the SMART device type: "nvme", "sata" or "scsi".
	DeviceType string `yaml:"device_type" protobuf:"2"`

	// Healthy is the overall SMART health verdict computed by Talos.
	Healthy bool `yaml:"healthy" protobuf:"3"`
	// Message carries additional details, e.g. why the disk is unhealthy or that
	// the probe was skipped because the disk was in standby.
	Message string `yaml:"message,omitempty" protobuf:"4"`

	// PowerState reports the disk power mode observed during the last probe, e.g.
	// "active", "idle" or "standby". SMART data is not refreshed while the disk is
	// in standby (to avoid spinning it up).
	PowerState string `yaml:"power_state,omitempty" protobuf:"5"`

	// Temperature is the disk temperature in degrees Celsius.
	Temperature  uint32 `yaml:"temperature,omitempty" protobuf:"6"`
	PowerOnHours uint64 `yaml:"power_on_hours,omitempty" protobuf:"7"`
	PowerCycles  uint64 `yaml:"power_cycles,omitempty" protobuf:"8"`

	// NVMe-specific fields.
	PercentUsed     uint32 `yaml:"percent_used,omitempty" protobuf:"9"`
	AvailableSpare  uint32 `yaml:"available_spare,omitempty" protobuf:"10"`
	CriticalWarning uint32 `yaml:"critical_warning,omitempty" protobuf:"11"`
	MediaErrors     uint64 `yaml:"media_errors,omitempty" protobuf:"12"`

	// Attributes is the raw ATA SMART attribute table (empty for NVMe/SCSI).
	Attributes []SMARTAttribute `yaml:"attributes,omitempty" protobuf:"13"`
}

// SMARTAttribute is a single ATA SMART attribute.
//
//gotagsrewrite:gen
type SMARTAttribute struct {
	ID        uint32 `yaml:"id" protobuf:"1"`
	Name      string `yaml:"name" protobuf:"2"`
	Current   uint32 `yaml:"current" protobuf:"3"`
	Worst     uint32 `yaml:"worst" protobuf:"4"`
	Threshold uint32 `yaml:"threshold" protobuf:"5"`
	RawValue  uint64 `yaml:"raw_value" protobuf:"6"`
	Failing   bool   `yaml:"failing,omitempty" protobuf:"7"`
}

// NewSMARTStatus initializes a SMARTStatus resource.
func NewSMARTStatus(namespace resource.Namespace, id resource.ID) *SMARTStatus {
	return typed.NewResource[SMARTStatusSpec, SMARTStatusExtension](
		resource.NewMetadata(namespace, SMARTStatusType, id, resource.VersionUndefined),
		SMARTStatusSpec{},
	)
}

// SMARTStatusExtension is auxiliary resource data for SMARTStatus.
type SMARTStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (SMARTStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SMARTStatusType,
		Aliases:          []resource.Type{"smart", "smartstatus"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Healthy",
				JSONPath: "{.healthy}",
			},
			{
				Name:     "Type",
				JSONPath: "{.device_type}",
			},
			{
				Name:     "Temperature",
				JSONPath: "{.temperature}",
			},
			{
				Name:     "Power On Hours",
				JSONPath: "{.power_on_hours}",
			},
			{
				Name:     "Percent Used",
				JSONPath: "{.percent_used}",
			},
			{
				Name:     "Media Errors",
				JSONPath: "{.media_errors}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(SMARTStatusType, &SMARTStatus{})
	if err != nil {
		panic(err)
	}
}
