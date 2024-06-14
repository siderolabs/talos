// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// StatusType is the type of Status resource.
const StatusType = resource.Type("SiderolinkStatuses.siderolink.talos.dev")

// StatusID the singleton status resource ID.
const StatusID = resource.ID("siderolink-status")

// Status resource holds Siderolink status.
type Status = typed.Resource[StatusSpec, StatusExtension]

// StatusSpec describes Siderolink status.
//
//gotagsrewrite:gen
type StatusSpec struct {
	// Host is the Siderolink target host.
	Host string `yaml:"host" protobuf:"1"`
	// Connected is the status of the Siderolink GRPC connection.
	Connected bool `yaml:"connected" protobuf:"2"`
}

// NewStatus initializes a Status resource.
func NewStatus() *Status {
	return typed.NewResource[StatusSpec, StatusExtension](
		resource.NewMetadata(config.NamespaceName, StatusType, StatusID, resource.VersionUndefined),
		StatusSpec{},
	)
}

// StatusExtension provides auxiliary methods for Status.
type StatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (StatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             StatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: config.NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Host",
				JSONPath: `{.host}`,
			},
			{
				Name:     "Connected",
				JSONPath: `{.connected}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[StatusSpec](StatusType, &Status{})
	if err != nil {
		panic(err)
	}
}
