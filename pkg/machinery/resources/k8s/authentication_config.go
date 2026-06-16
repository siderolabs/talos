// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// AuthenticationConfigType is type of AuthenticationConfig resource.
const AuthenticationConfigType = resource.Type("AuthenticationConfigs.kubernetes.talos.dev")

// AuthenticationConfigID is a singleton resource ID for AuthenticationConfig.
const AuthenticationConfigID = resource.ID("authentication-config")

// AuthenticationConfig represents configuration for kube-apiserver authentication configuration.
type AuthenticationConfig = typed.Resource[AuthenticationConfigSpec, AuthenticationConfigExtension]

// AuthenticationConfigSpec is authentication configuration for kube-apiserver.
//
//gotagsrewrite:gen
type AuthenticationConfigSpec struct {
	Config map[string]any `yaml:"config" protobuf:"1"`
}

// NewAuthenticationConfig returns new AuthenticationConfig resource.
func NewAuthenticationConfig() *AuthenticationConfig {
	return typed.NewResource[AuthenticationConfigSpec, AuthenticationConfigExtension](
		resource.NewMetadata(ControlPlaneNamespaceName, AuthenticationConfigType, AuthenticationConfigID, resource.VersionUndefined),
		AuthenticationConfigSpec{},
	)
}

// AuthenticationConfigExtension defines AuthenticationConfig resource definition.
type AuthenticationConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (AuthenticationConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AuthenticationConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[AuthenticationConfigSpec](AuthenticationConfigType, &AuthenticationConfig{})
	if err != nil {
		panic(err)
	}
}
