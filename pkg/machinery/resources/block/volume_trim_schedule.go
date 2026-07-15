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

// VolumeTrimScheduleType is type of VolumeTrimSchedule resource.
const VolumeTrimScheduleType = resource.Type("VolumeTrimSchedules.block.talos.dev")

// VolumeTrimSchedule resource describes when a volume should be trimmed (fstrim).
//
// The resource ID is the volume ID.
type VolumeTrimSchedule = typed.Resource[VolumeTrimScheduleSpec, VolumeTrimScheduleExtension]

// VolumeTrimScheduleSpec is the spec for VolumeTrimSchedule resource.
//
//gotagsrewrite:gen
type VolumeTrimScheduleSpec struct {
	// Filesystem is the filesystem type of the volume to be trimmed.
	Filesystem FilesystemType `yaml:"filesystem" protobuf:"1"`
	// Interval is the trim interval for the volume.
	Interval time.Duration `yaml:"interval" protobuf:"2"`
	// NextTrim is the next scheduled trim time for the volume.
	NextTrim time.Time `yaml:"nextTrim" protobuf:"3"`
}

// NewVolumeTrimSchedule initializes a VolumeTrimSchedule resource.
func NewVolumeTrimSchedule(namespace resource.Namespace, id resource.ID) *VolumeTrimSchedule {
	return typed.NewResource[VolumeTrimScheduleSpec, VolumeTrimScheduleExtension](
		resource.NewMetadata(namespace, VolumeTrimScheduleType, id, resource.VersionUndefined),
		VolumeTrimScheduleSpec{},
	)
}

// VolumeTrimScheduleExtension is auxiliary resource data for VolumeTrimSchedule.
type VolumeTrimScheduleExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (VolumeTrimScheduleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             VolumeTrimScheduleType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Filesystem",
				JSONPath: `{.filesystem}`,
			},
			{
				Name:     "Interval",
				JSONPath: `{.interval}`,
			},
			{
				Name:     "Next Trim",
				JSONPath: `{.nextTrim}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(VolumeTrimScheduleType, &VolumeTrimSchedule{})
	if err != nil {
		panic(err)
	}
}
