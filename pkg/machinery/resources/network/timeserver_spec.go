// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// TimeServerSpecType is type of TimeServerSpec resource.
const TimeServerSpecType = resource.Type("TimeServerSpecs.net.talos.dev")

// TimeServerSpec resource holds NTP server info.
type TimeServerSpec = typed.Resource[TimeServerSpecSpec, TimeServerSpecRD]

// TimeServerID is the ID of the singleton instance.
const TimeServerID resource.ID = "timeservers"

// TimeServerSpecSpec describes NTP servers.
type TimeServerSpecSpec struct {
	NTPServers  []string    `yaml:"timeServers"`
	ConfigLayer ConfigLayer `yaml:"layer"`
}

// NewTimeServerSpec initializes a TimeServerSpec resource.
func NewTimeServerSpec(namespace resource.Namespace, id resource.ID) *TimeServerSpec {
	return typed.NewResource[TimeServerSpecSpec, TimeServerSpecRD](
		resource.NewMetadata(namespace, TimeServerSpecType, id, resource.VersionUndefined),
		TimeServerSpecSpec{},
	)
}

// TimeServerSpecRD provides auxiliary methods for TimeServerSpec.
type TimeServerSpecRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (TimeServerSpecRD) ResourceDefinition(resource.Metadata, TimeServerSpecSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TimeServerSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}
