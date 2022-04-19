// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// TimeServerStatusType is type of TimeServerStatus resource.
const TimeServerStatusType = resource.Type("TimeServerStatuses.net.talos.dev")

// TimeServerStatus resource holds NTP server info.
type TimeServerStatus = typed.Resource[TimeServerStatusSpec, TimeServerStatusRD]

// TimeServerStatusSpec describes NTP servers.
type TimeServerStatusSpec struct {
	NTPServers []string `yaml:"timeServers"`
}

// DeepCopy generates a deep copy of TimeServerStatusSpec.
func (spec TimeServerStatusSpec) DeepCopy() TimeServerStatusSpec {
	cp := spec
	if spec.NTPServers != nil {
		cp.NTPServers = make([]string, len(spec.NTPServers))
		copy(cp.NTPServers, spec.NTPServers)
	}

	return cp
}

// NewTimeServerStatus initializes a TimeServerStatus resource.
func NewTimeServerStatus(namespace resource.Namespace, id resource.ID) *TimeServerStatus {
	return typed.NewResource[TimeServerStatusSpec, TimeServerStatusRD](
		resource.NewMetadata(namespace, TimeServerStatusType, id, resource.VersionUndefined),
		TimeServerStatusSpec{},
	)
}

// TimeServerStatusRD provides auxiliary methods for TimeServerStatus.
type TimeServerStatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (TimeServerStatusRD) ResourceDefinition(resource.Metadata, TimeServerStatusSpec) meta.ResourceDefinitionSpec {
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
