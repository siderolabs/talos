// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
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

// FSScrubSchedule resource describes when a volume filesystem should be scrubbed.
//
// The resource ID is the volume ID.
type FSScrubSchedule = typed.Resource[FSScrubScheduleSpec, FSScrubScheduleExtension]

// FSScrubScheduleSpec is the spec for FSScrubSchedule resource.
//
//gotagsrewrite:gen
type FSScrubScheduleSpec struct {
	// Filesystem is the filesystem type of the volume to be scrubbed.
	Filesystem FilesystemType `yaml:"filesystem" protobuf:"1"`
	// Interval is the scrub interval for the volume.
	Interval time.Duration `yaml:"interval" protobuf:"2"`
	// NextScrub is the next scheduled scrub time for the volume.
	NextScrub time.Time `yaml:"nextScrub" protobuf:"3"`
}

// NewFSScrubSchedule initializes a FSScrubSchedule resource.
func NewFSScrubSchedule(namespace resource.Namespace, id resource.ID) *FSScrubSchedule {
	return typed.NewResource[FSScrubScheduleSpec, FSScrubScheduleExtension](
		resource.NewMetadata(namespace, FSScrubScheduleType, id, resource.VersionUndefined),
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
				Name:     "Filesystem",
				JSONPath: `{.filesystem}`,
			},
			{
				Name:     "Interval",
				JSONPath: `{.interval}`,
			},
			{
				Name:     "Next Scrub",
				JSONPath: `{.nextScrub}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(FSScrubScheduleType, &FSScrubSchedule{})
	if err != nil {
		panic(err)
	}
}
