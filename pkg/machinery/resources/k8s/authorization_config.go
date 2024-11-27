// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package k8s provides resources which interface with Kubernetes.
package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// AuthorizationConfigType is type of AuthorizationConfig resource.
const AuthorizationConfigType = resource.Type("AuthorizationConfigs.kubernetes.talos.dev")

// AuthorizationConfigID is a singleton resource ID for AuthorizationConfig.
const AuthorizationConfigID = resource.ID("authorization")

// AuthorizationConfig represents configuration for kube-apiserver authorization.
type AuthorizationConfig = typed.Resource[AuthorizationConfigSpec, AuthorizationConfigExtension]

// AuthorizationConfigSpec is authorization configuration for kube-apiserver.
//
//gotagsrewrite:gen
type AuthorizationConfigSpec struct {
	Image  string                         `yaml:"image" protobuf:"1"`
	Config []AuthorizationAuthorizersSpec `yaml:"config" protobuf:"2"`
}

// AuthorizationAuthorizersSpec is a configuration of authorization authorizers.
//
//gotagsrewrite:gen
type AuthorizationAuthorizersSpec struct {
	Type    string         `yaml:"type" protobuf:"1"`
	Name    string         `yaml:"name" protobuf:"2"`
	Webhook map[string]any `yaml:"webhook" protobuf:"3"`
}

// NewAuthorizationConfig returns new AuthorizationConfig resource.
func NewAuthorizationConfig() *AuthorizationConfig {
	return typed.NewResource[AuthorizationConfigSpec, AuthorizationConfigExtension](
		resource.NewMetadata(ControlPlaneNamespaceName, AuthorizationConfigType, AuthorizationConfigID, resource.VersionUndefined),
		AuthorizationConfigSpec{})
}

// AuthorizationConfigExtension defines AuthorizationConfig resource definition.
type AuthorizationConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (AuthorizationConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AuthorizationConfigType,
		DefaultNamespace: ControlPlaneNamespaceName,
	}
}

func init() {
	err := protobuf.RegisterDynamic[AuthorizationConfigSpec](AuthorizationConfigType, &AuthorizationConfig{})
	if err != nil {
		panic(err)
	}
}
