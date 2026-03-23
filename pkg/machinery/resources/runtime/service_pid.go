// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ServicePIDType is type of [ServicePID] resource.
const ServicePIDType = resource.Type("ServicePIDs.runtime.talos.dev")

// ServicePID resource appears when all meta keys are loaded.
type ServicePID = typed.Resource[ServicePIDSpec, ServicePIDExtension]

// ServicePIDSpec is the spec for the service PID.
//
//gotagsrewrite:gen
type ServicePIDSpec struct {
	PID int32 `yaml:"pid" protobuf:"1"`
}

// NewServicePID initializes a [ServicePID] resource.
func NewServicePID(id string) *ServicePID {
	return typed.NewResource[ServicePIDSpec, ServicePIDExtension](
		resource.NewMetadata(NamespaceName, ServicePIDType, id, resource.VersionUndefined),
		ServicePIDSpec{},
	)
}

// ServicePIDExtension is auxiliary resource data for [ServicePID].
type ServicePIDExtension struct{}

// ResourceDefinition implements [meta.ResourceDefinitionProvider] interface.
func (ServicePIDExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ServicePIDType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "PID",
				JSONPath: `{.pid}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[ServicePIDSpec](ServicePIDType, &ServicePID{})
	if err != nil {
		panic(err)
	}
}
