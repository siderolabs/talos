// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
)

// StatusType is type of TimeSync resource.
const StatusType = resource.Type("TimeStatuses.v1alpha1.talos.dev")

// StatusID is the ID of the singletone resource.
const StatusID = resource.ID("node")

// Status describes running current time sync status.
type Status struct {
	md   resource.Metadata
	spec StatusSpec
}

// StatusSpec describes time sync state.
type StatusSpec struct {
	// Synced indicates whether time is in sync.
	Synced bool `yaml:"synced"`

	// Epoch is incremented every time clock jumps more than 15min.
	Epoch int `yaml:"epoch"`

	// SyncDisabled indicates if time sync is disabled.
	SyncDisabled bool `yaml:"syncDisabled"`
}

// NewStatus initializes a TimeSync resource.
func NewStatus() *Status {
	r := &Status{
		md:   resource.NewMetadata(v1alpha1.NamespaceName, StatusType, StatusID, resource.VersionUndefined),
		spec: StatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Status) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Status) Spec() interface{} {
	return r.spec
}

func (r *Status) String() string {
	return fmt.Sprintf("time.Status(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Status) DeepCopy() resource.Resource {
	return &Status{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Status) ResourceDefinition() meta.ResourceDefinitionSpec {
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

// SetStatus changes .spec.
func (r *Status) SetStatus(status StatusSpec) {
	r.spec = status
}

// Status returns .spec.
func (r *Status) Status() StatusSpec {
	return r.spec
}
