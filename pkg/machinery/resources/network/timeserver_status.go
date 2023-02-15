// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// TimeServerStatusType is type of TimeServerStatus resource.
const TimeServerStatusType = resource.Type("TimeServerStatuses.net.talos.dev")

// TimeServerStatus resource holds NTP server info.
type TimeServerStatus = typed.Resource[TimeServerStatusSpec, TimeServerStatusRD]

// TimeServerStatusSpec describes NTP servers.
//
//gotagsrewrite:gen
type TimeServerStatusSpec struct {
	NTPServers []string `yaml:"timeServers" protobuf:"1"`
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
func (TimeServerStatusRD) ResourceDefinition() meta.ResourceDefinitionSpec {
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

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[TimeServerStatusSpec](TimeServerStatusType, &TimeServerStatus{})
	if err != nil {
		panic(err)
	}
}
