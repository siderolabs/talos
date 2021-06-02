// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// TimeServerStatusType is type of TimeServerStatus resource.
const TimeServerStatusType = resource.Type("TimeServerStatuses.net.talos.dev")

// TimeServerStatus resource holds NTP server info.
type TimeServerStatus struct {
	md   resource.Metadata
	spec TimeServerStatusSpec
}

// TimeServerStatusSpec describes NTP servers.
type TimeServerStatusSpec struct {
	NTPServers []string `yaml:"timeServers"`
}

// NewTimeServerStatus initializes a TimeServerStatus resource.
func NewTimeServerStatus(namespace resource.Namespace, id resource.ID) *TimeServerStatus {
	r := &TimeServerStatus{
		md:   resource.NewMetadata(namespace, TimeServerStatusType, id, resource.VersionUndefined),
		spec: TimeServerStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *TimeServerStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *TimeServerStatus) Spec() interface{} {
	return r.spec
}

func (r *TimeServerStatus) String() string {
	return fmt.Sprintf("network.TimeServerStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *TimeServerStatus) DeepCopy() resource.Resource {
	return &TimeServerStatus{
		md: r.md,
		spec: TimeServerStatusSpec{
			NTPServers: append([]string(nil), r.spec.NTPServers...),
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *TimeServerStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TimeServerStatusType,
		Aliases:          []resource.Type{"timeserver", "timeservers"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Timeservers",
				JSONPath: "{.timeServers}",
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *TimeServerStatus) TypedSpec() *TimeServerStatusSpec {
	return &r.spec
}
