// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/crypto/x509"

	config2 "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// RegistriesConfigType is type of RegistriesConfig resource.
const RegistriesConfigType = resource.Type("RegistryConfigs.cri.talos.dev")

// RegistriesConfigID is the singleton resource ID.
const RegistriesConfigID resource.ID = "registries"

// RegistriesConfig resource holds info about container registries.
type RegistriesConfig = typed.Resource[RegistriesConfigSpec, RegistriesConfigExtension]

// RegistriesConfigSpec describes status of rendered secrets.
//
//gotagsrewrite:gen
type RegistriesConfigSpec struct {
	RegistryMirrors map[string]*RegistryMirrorConfig `yaml:"mirrors,omitempty" protobuf:"1"`
	RegistryConfig  map[string]*RegistryConfig       `yaml:"config,omitempty" protobuf:"2"`
}

// Mirrors implements the Registries interface.
func (r RegistriesConfigSpec) Mirrors() map[string]config2.RegistryMirrorConfig {
	mirrors := make(map[string]config2.RegistryMirrorConfig, len(r.RegistryMirrors))

	for k, v := range r.RegistryMirrors {
		mirrors[k] = (*v1alpha1.RegistryMirrorConfig)(v)
	}

	return mirrors
}

// Config implements the Registries interface.
func (r RegistriesConfigSpec) Config() map[string]config2.RegistryConfig {
	registries := make(map[string]config2.RegistryConfig, len(r.RegistryConfig))

	for k, v := range r.RegistryConfig {
		registries[k] = v
	}

	return registries
}

// RegistryMirrorConfig represents mirror configuration for a registry.
//
//gotagsrewrite:gen
type RegistryMirrorConfig struct {
	MirrorEndpoints    []string `yaml:"endpoints" protobuf:"1"`
	MirrorOverridePath *bool    `yaml:"overridePath,omitempty" protobuf:"2"`
	MirrorSkipFallback *bool    `yaml:"skipFallback,omitempty" protobuf:"3"`
}

// RegistryConfig specifies auth & TLS config per registry.
//
//gotagsrewrite:gen
type RegistryConfig struct {
	RegistryTLS  *RegistryTLSConfig  `yaml:"tls,omitempty" protobuf:"1"`
	RegistryAuth *RegistryAuthConfig `yaml:"auth,omitempty" protobuf:"2"`
}

// TLS implements the Registries interface.
func (c *RegistryConfig) TLS() config2.RegistryTLSConfig {
	if c.RegistryTLS == nil {
		return nil
	}

	return (*v1alpha1.RegistryTLSConfig)(c.RegistryTLS)
}

// Auth implements the Registries interface.
func (c *RegistryConfig) Auth() config2.RegistryAuthConfig {
	if c.RegistryAuth == nil {
		return nil
	}

	return (*v1alpha1.RegistryAuthConfig)(c.RegistryAuth)
}

// RegistryAuthConfig specifies authentication configuration for a registry.
//
//gotagsrewrite:gen
type RegistryAuthConfig struct {
	RegistryUsername      string `yaml:"username,omitempty" protobuf:"1"`
	RegistryPassword      string `yaml:"password,omitempty" protobuf:"2"`
	RegistryAuth          string `yaml:"auth,omitempty" protobuf:"3"`
	RegistryIdentityToken string `yaml:"identityToken,omitempty" protobuf:"4"`
}

// RegistryTLSConfig specifies TLS config for HTTPS registries.
//
//gotagsrewrite:gen
type RegistryTLSConfig struct {
	TLSClientIdentity     *x509.PEMEncodedCertificateAndKey `yaml:"clientIdentity,omitempty" protobuf:"1"`
	TLSCA                 v1alpha1.Base64Bytes              `yaml:"ca,omitempty" protobuf:"2"`
	TLSInsecureSkipVerify *bool                             `yaml:"insecureSkipVerify,omitempty" protobuf:"3"`
}

// NewRegistriesConfig initializes a RegistriesConfig resource.
func NewRegistriesConfig() *RegistriesConfig {
	return typed.NewResource[RegistriesConfigSpec, RegistriesConfigExtension](
		resource.NewMetadata(NamespaceName, RegistriesConfigType, RegistriesConfigID, resource.VersionUndefined),
		RegistriesConfigSpec{},
	)
}

// RegistriesConfigExtension provides auxiliary methods for RegistriesConfig.
type RegistriesConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (RegistriesConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RegistriesConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[RegistriesConfigSpec](RegistriesConfigType, &RegistriesConfig{})
	if err != nil {
		panic(err)
	}
}
