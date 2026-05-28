// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// DiskHealthStatusType is type of DiskHealthStatus resource.
const DiskHealthStatusType = resource.Type("DiskHealthStatuses.block.talos.dev")

// DiskHealthStatus resource holds disk health information.
type DiskHealthStatus = typed.Resource[DiskHealthStatusSpec, DiskHealthStatusExtension]

// DiskHealthStatusSpec is the spec for DiskHealthStatus resource.
//
//gotagsrewrite:gen
type DiskHealthStatusSpec struct {
	DiskID string `yaml:"diskID" protobuf:"1"`
	Device string `yaml:"device" protobuf:"2"`

	HealthSource       DiskHealthSource      `yaml:"healthSource" protobuf:"3"`
	Status             DiskHealthStatusValue `yaml:"status" protobuf:"4"`
	TemperatureCelsius int32                 `yaml:"temperatureCelsius" protobuf:"5"`
	PowerOnHours       uint64                `yaml:"powerOnHours" protobuf:"6"`
	PowerCycles        uint64                `yaml:"powerCycles" protobuf:"7"`
	LastChecked        time.Time             `yaml:"lastChecked" protobuf:"8"`
	Error              string                `yaml:"error,omitempty" protobuf:"9"`

	Details DiskHealthDetails `yaml:"details" protobuf:"10"`
}

// DiskHealthDetails contains backend-specific health details.
//
//gotagsrewrite:gen
type DiskHealthDetails struct {
	NVMe *DiskHealthNVMeDetails `yaml:"nvme,omitempty" protobuf:"1"`
	ATA  *DiskHealthATADetails  `yaml:"ata,omitempty" protobuf:"2"`
}

// DiskHealthNVMeDetails contains NVMe-specific health information.
//
//gotagsrewrite:gen
type DiskHealthNVMeDetails struct {
	CriticalWarning             uint32 `yaml:"criticalWarning" protobuf:"1"`
	PercentageUsed              uint32 `yaml:"percentageUsed" protobuf:"2"`
	UnsafeShutdowns             uint64 `yaml:"unsafeShutdowns" protobuf:"3"`
	MediaAndDataIntegrityErrors uint64 `yaml:"mediaAndDataIntegrityErrors" protobuf:"4"`
}

// DiskHealthATADetails contains ATA SMART-specific health information.
//
//gotagsrewrite:gen
type DiskHealthATADetails struct {
	ReallocatedSectorCount      uint64 `yaml:"reallocatedSectorCount" protobuf:"1"`
	CurrentPendingSectorCount   uint64 `yaml:"currentPendingSectorCount" protobuf:"2"`
	OfflineUncorrectableCount   uint64 `yaml:"offlineUncorrectableCount" protobuf:"3"`
	ReportedUncorrectableErrors uint64 `yaml:"reportedUncorrectableErrors" protobuf:"4"`
	WearLevelingCount           uint64 `yaml:"wearLevelingCount" protobuf:"5"`
}

// NewDiskHealthStatus initializes a DiskHealthStatus resource.
func NewDiskHealthStatus(namespace resource.Namespace, id resource.ID) *DiskHealthStatus {
	return typed.NewResource[DiskHealthStatusSpec, DiskHealthStatusExtension](
		resource.NewMetadata(namespace, DiskHealthStatusType, id, resource.VersionUndefined),
		DiskHealthStatusSpec{},
	)
}

// DiskHealthStatusExtension is auxiliary resource data for DiskHealthStatus.
type DiskHealthStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (DiskHealthStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DiskHealthStatusType,
		Aliases:          []resource.Type{"diskhealth", "dhs"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Status",
				JSONPath: "{.status}",
			},
			{
				Name:     "Source",
				JSONPath: "{.healthSource}",
			},
			{
				Name:     "Temp",
				JSONPath: "{.temperatureCelsius}",
			},
			{
				Name:     "Power Hours",
				JSONPath: "{.powerOnHours}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(DiskHealthStatusType, &DiskHealthStatus{})
	if err != nil {
		panic(err)
	}
}
