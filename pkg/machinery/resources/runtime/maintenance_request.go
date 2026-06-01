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

// MaintenanceServiceRequestType is type of MaintenanceServiceConfig resource.
const MaintenanceServiceRequestType = resource.Type("MaintenanceServiceRequests.runtime.talos.dev")

// MaintenanceServiceRequest resource indicates that the maintenance service should run.
type MaintenanceServiceRequest = typed.Resource[MaintenanceServiceRequestSpec, MaintenanceServiceRequestExtension]

// MaintenanceServiceRequestID is a resource ID for MaintenanceConfig.
const MaintenanceServiceRequestID resource.ID = "maintenance"

// MaintenanceServiceRequestSpec indicates that maintenance service API should be started.
//
//gotagsrewrite:gen
type MaintenanceServiceRequestSpec struct{}

// NewMaintenanceServiceRequest initializes a MaintenanceConfig resource.
func NewMaintenanceServiceRequest() *MaintenanceServiceRequest {
	return typed.NewResource[MaintenanceServiceRequestSpec, MaintenanceServiceRequestExtension](
		resource.NewMetadata(NamespaceName, MaintenanceServiceRequestType, MaintenanceServiceRequestID, resource.VersionUndefined),
		MaintenanceServiceRequestSpec{},
	)
}

// MaintenanceServiceRequestExtension is auxiliary resource data for MaintenanceConfig.
type MaintenanceServiceRequestExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MaintenanceServiceRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MaintenanceServiceRequestType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MaintenanceServiceRequestSpec](MaintenanceServiceRequestType, &MaintenanceServiceRequest{})
	if err != nil {
		panic(err)
	}
}
