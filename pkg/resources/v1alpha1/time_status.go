// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/resource/meta"
)

// TimeStatusType is type of TimeSync resource.
const TimeStatusType = resource.Type("TimeStatuses.v1alpha1.talos.dev")

// TimeStatusID is the ID of the singletone resource.
const TimeStatusID = resource.ID("node")

// TimeStatus describes running current time sync status.
type TimeStatus struct {
	md   resource.Metadata
	spec TimeStatusSpec
}

// TimeStatusSpec describes time sync state.
type TimeStatusSpec struct {
	Synced bool `yaml:"synced"`
}

// NewTimeStatus initializes a TimeSync resource.
func NewTimeStatus() *TimeStatus {
	r := &TimeStatus{
		md:   resource.NewMetadata(NamespaceName, TimeStatusType, TimeStatusID, resource.VersionUndefined),
		spec: TimeStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *TimeStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *TimeStatus) Spec() interface{} {
	return r.spec
}

func (r *TimeStatus) String() string {
	return fmt.Sprintf("v1alpha1.TimeStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *TimeStatus) DeepCopy() resource.Resource {
	return &TimeStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *TimeStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TimeStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Synced",
				JSONPath: "{.synced}",
			},
		},
	}
}

// SetSynced changes .spec.synced.
func (r *TimeStatus) SetSynced(sync bool) {
	r.spec.Synced = sync
}

// Synced returns .spec.synced.
func (r *TimeStatus) Synced() bool {
	return r.spec.Synced
}
