// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
)

//nolint:lll
//go:generate deep-copy -type StatusSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// StatusType is type of TimeSync resource.
const StatusType = resource.Type("TimeStatuses.v1alpha1.talos.dev")

// StatusID is the ID of the singletone resource.
const StatusID = resource.ID("node")

// Status describes running current time sync status.
type Status = typed.Resource[StatusSpec, StatusRD]

// StatusSpec describes time sync state.
//
//gotagsrewrite:gen
type StatusSpec struct {
	// Synced indicates whether time is in sync.
	Synced bool `yaml:"synced" protobuf:"1"`

	// Epoch is incremented every time clock jumps more than 15min.
	Epoch int `yaml:"epoch" protobuf:"2"`

	// SyncDisabled indicates if time sync is disabled.
	SyncDisabled bool `yaml:"syncDisabled" protobuf:"3"`
}

// NewStatus initializes a TimeSync resource.
func NewStatus() *Status {
	return typed.NewResource[StatusSpec, StatusRD](
		resource.NewMetadata(v1alpha1.NamespaceName, StatusType, StatusID, resource.VersionUndefined),
		StatusSpec{},
	)
}

// StatusRD provides auxiliary methods for Status.
type StatusRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r StatusRD) ResourceDefinition(resource.Metadata, StatusSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: v1alpha1.NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Synced",
				JSONPath: "{.synced}",
			},
		},
	}
}
