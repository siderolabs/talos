// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// MaintenanceServiceConfigType is type of MaintenanceConfig resource.
const MaintenanceServiceConfigType = resource.Type("MaintenanceServiceConfigs.runtime.talos.dev")

// MaintenanceServiceConfig resource holds configuration for maintenance service API.
type MaintenanceServiceConfig = typed.Resource[MaintenanceServiceConfigSpec, MaintenanceServiceConfigExtension]

// MaintenanceServiceConfigID is a resource ID for MaintenanceConfig.
const MaintenanceServiceConfigID resource.ID = "maintenance"

// MaintenanceServiceConfigSpec describes configuration for maintenance service API.
//
//gotagsrewrite:gen
type MaintenanceServiceConfigSpec struct {
	ListenAddress      string       `yaml:"listenAddress" protobuf:"1"`
	ReachableAddresses []netip.Addr `yaml:"reachableAddresses" protobuf:"2"`
}

// NewMaintenanceServiceConfig initializes a MaintenanceConfig resource.
func NewMaintenanceServiceConfig() *MaintenanceServiceConfig {
	return typed.NewResource[MaintenanceServiceConfigSpec, MaintenanceServiceConfigExtension](
		resource.NewMetadata(NamespaceName, MaintenanceServiceConfigType, MaintenanceServiceConfigID, resource.VersionUndefined),
		MaintenanceServiceConfigSpec{},
	)
}

// MaintenanceServiceConfigExtension is auxiliary resource data for MaintenanceConfig.
type MaintenanceServiceConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MaintenanceServiceConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MaintenanceServiceConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[MaintenanceServiceConfigSpec](MaintenanceServiceConfigType, &MaintenanceServiceConfig{})
	if err != nil {
		panic(err)
	}
}
