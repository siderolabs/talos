// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/resource/core"
)

// TimeSyncType is type of TimeSync resource.
const TimeSyncType = resource.Type("v1alpha1/timeSync")

// TimeSyncID is the ID of the singletone resource.
const TimeSyncID = resource.ID("timeSync")

// TimeSync describes running current time sync status.
type TimeSync struct {
	md   resource.Metadata
	spec TimeSyncSpec
}

// TimeSyncSpec describes time sync state.
type TimeSyncSpec struct {
	Sync bool `yaml:"sync"`
}

// NewTimeSync initializes a TimeSync resource.
func NewTimeSync() *TimeSync {
	r := &TimeSync{
		md:   resource.NewMetadata(NamespaceName, TimeSyncType, TimeSyncID, resource.VersionUndefined),
		spec: TimeSyncSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *TimeSync) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *TimeSync) Spec() interface{} {
	return r.spec
}

func (r *TimeSync) String() string {
	return fmt.Sprintf("v1alpha1.TimeSync(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *TimeSync) DeepCopy() resource.Resource {
	return &TimeSync{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (r *TimeSync) ResourceDefinition() core.ResourceDefinitionSpec {
	return core.ResourceDefinitionSpec{
		Type:             TimeSyncType,
		Aliases:          []resource.Type{"timeSync"},
		DefaultNamespace: NamespaceName,
	}
}

// SetSync changes .spec.sync.
func (r *TimeSync) SetSync(sync bool) {
	r.spec.Sync = sync
}

// Sync returns .spec.sync.
func (r *TimeSync) Sync() bool {
	return r.spec.Sync
}
