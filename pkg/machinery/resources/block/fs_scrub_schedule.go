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

// FSScrubScheduleType is type of FSScrubSchedule resource.
const FSScrubScheduleType = resource.Type("FSScrubSchedules.block.talos.dev")

// FSScrubSchedule resource holds status of watchdog timer.
type FSScrubSchedule = typed.Resource[FSScrubScheduleSpec, FSScrubScheduleExtension]

// FSScrubScheduleSpec describes scheduled filesystem scrubbing jobs.
//
//gotagsrewrite:gen
type FSScrubScheduleSpec struct {
	Mountpoint string        `yaml:"mountpoint" protobuf:"1"`
	Period     time.Duration `yaml:"period" protobuf:"2"`
	StartTime  time.Time     `yaml:"startTime" protobuf:"3"`
}

// NewFSScrubSchedule initializes a FSScrubSchedule resource.
func NewFSScrubSchedule(id string) *FSScrubSchedule {
	return typed.NewResource[FSScrubScheduleSpec, FSScrubScheduleExtension](
		resource.NewMetadata(NamespaceName, FSScrubScheduleType, id, resource.VersionUndefined),
		FSScrubScheduleSpec{},
	)
}

// FSScrubScheduleExtension is auxiliary resource data for FSScrubSchedule.
type FSScrubScheduleExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (FSScrubScheduleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             FSScrubScheduleType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Mountpoint",
				JSONPath: `{.mountpoint}`,
			},
			{
				Name:     "Period",
				JSONPath: `{.period}`,
			},
			{
				Name:     "First start time",
				JSONPath: `{.startTime}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[FSScrubScheduleSpec](FSScrubScheduleType, &FSScrubSchedule{})
	if err != nil {
		panic(err)
	}
}
