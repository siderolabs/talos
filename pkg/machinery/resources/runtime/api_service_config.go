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

// APIServiceConfigType is type of MaintenanceConfig resource.
const APIServiceConfigType = resource.Type("APIServiceConfigs.runtime.talos.dev")

// APIServiceConfig resource holds configuration for Talos API service (apid).
type APIServiceConfig = typed.Resource[APIServiceConfigSpec, APIServiceConfigExtension]

// APIServiceConfigID is a resource ID for APIServiceConfig.
const APIServiceConfigID resource.ID = "apid"

// APIServiceConfigSpec describes configuration for Talos API service (apid).
//
//gotagsrewrite:gen
type APIServiceConfigSpec struct {
	ListenAddress string `yaml:"listenAddress" protobuf:"1"`

	NodeRoutingDisabled     bool `yaml:"nodeRoutingDisabled" protobuf:"2"`
	ReadonlyRoleMode        bool `yaml:"readonlyRoleMode" protobuf:"3"`
	SkipVerifyingClientCert bool `yaml:"skipVerifyingClientCert" protobuf:"4"`
}

// NewAPIServiceConfig initializes an APIServiceConfig resource.
func NewAPIServiceConfig() *APIServiceConfig {
	return typed.NewResource[APIServiceConfigSpec, APIServiceConfigExtension](
		resource.NewMetadata(NamespaceName, APIServiceConfigType, APIServiceConfigID, resource.VersionUndefined),
		APIServiceConfigSpec{},
	)
}

// APIServiceConfigExtension is auxiliary resource data for APIServiceConfig.
type APIServiceConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (APIServiceConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             APIServiceConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[APIServiceConfigSpec](APIServiceConfigType, &APIServiceConfig{})
	if err != nil {
		panic(err)
	}
}
