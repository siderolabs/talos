// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time

import (
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// AdjtimeStatusType is type of AdjtimeStatus resource.
const AdjtimeStatusType = resource.Type("AdjtimeStatuses.v1alpha1.talos.dev")

// AdjtimeStatusID is the ID of the singletone resource.
const AdjtimeStatusID = resource.ID("node")

// AdjtimeStatus describes running current time sync AdjtimeStatus.
type AdjtimeStatus = typed.Resource[AdjtimeStatusSpec, AdjtimeStatusExtension]

// AdjtimeStatusSpec describes Linux internal adjtime state.
//
//gotagsrewrite:gen
type AdjtimeStatusSpec struct {
	Offset                   time.Duration `yaml:"offset" protobuf:"1"`
	FrequencyAdjustmentRatio float64       `yaml:"frequencyAdjustmentRatio" protobuf:"2"`
	MaxError                 time.Duration `yaml:"maxError" protobuf:"3"`
	EstError                 time.Duration `yaml:"estError" protobuf:"4"`
	Status                   string        `yaml:"status" protobuf:"5"`
	Constant                 int           `yaml:"constant" protobuf:"6"`
	SyncStatus               bool          `yaml:"syncStatus" protobuf:"7"`
	State                    string        `yaml:"state" protobuf:"8"`
}

// NewAdjtimeStatus initializes a TimeSync resource.
func NewAdjtimeStatus() *AdjtimeStatus {
	return typed.NewResource[AdjtimeStatusSpec, AdjtimeStatusExtension](
		resource.NewMetadata(v1alpha1.NamespaceName, AdjtimeStatusType, AdjtimeStatusID, resource.VersionUndefined),
		AdjtimeStatusSpec{},
	)
}

// AdjtimeStatusExtension provides auxiliary methods for AdjtimeStatus.
type AdjtimeStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (AdjtimeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AdjtimeStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: v1alpha1.NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Offset",
				JSONPath: "{.offset}",
			},
			{
				Name:     "EstError",
				JSONPath: "{.estError}",
			},
			{
				Name:     "MaxError",
				JSONPath: "{.maxError}",
			},
			{
				Name:     "Status",
				JSONPath: "{.status}",
			},
			{
				Name:     "Sync",
				JSONPath: "{.syncStatus}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[AdjtimeStatusSpec](AdjtimeStatusType, &AdjtimeStatus{})
	if err != nil {
		panic(err)
	}
}
