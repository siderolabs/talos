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

// FSScrubStatusType is type of FSScrubStatus resource.
const FSScrubStatusType = resource.Type("FSScrubStatuses.block.talos.dev")

// FSScrubStatus resource holds the result of the most recent filesystem scrub of a volume.
//
// The resource ID is the volume ID.
type FSScrubStatus = typed.Resource[FSScrubStatusSpec, FSScrubStatusExtension]

// FSScrubStatusSpec describes status of filesystem scrub jobs.
//
//gotagsrewrite:gen
type FSScrubStatusSpec struct {
	Mountpoint string        `yaml:"mountpoint" protobuf:"1"`
	Interval   time.Duration `yaml:"interval" protobuf:"2"`
	Time       time.Time     `yaml:"time" protobuf:"3"`
	Duration   time.Duration `yaml:"duration" protobuf:"4"`
	Status     string        `yaml:"status" protobuf:"5"`
}

// NewFSScrubStatus initializes a FSScrubStatus resource.
func NewFSScrubStatus(id string) *FSScrubStatus {
	return typed.NewResource[FSScrubStatusSpec, FSScrubStatusExtension](
		resource.NewMetadata(NamespaceName, FSScrubStatusType, id, resource.VersionUndefined),
		FSScrubStatusSpec{},
	)
}

// FSScrubStatusExtension is auxiliary resource data for FSScrubStatus.
type FSScrubStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (FSScrubStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             FSScrubStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Mountpoint",
				JSONPath: `{.mountpoint}`,
			},
			{
				Name:     "Interval",
				JSONPath: `{.interval}`,
			},
			{
				Name:     "Latest start time",
				JSONPath: `{.time}`,
			},
			{
				Name:     "Latest run duration",
				JSONPath: `{.duration}`,
			},
			{
				Name:     "Latest status",
				JSONPath: `{.status}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[FSScrubStatusSpec](FSScrubStatusType, &FSScrubStatus{})
	if err != nil {
		panic(err)
	}
}
