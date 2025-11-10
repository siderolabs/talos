// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"crypto/tls"
	stdx509 "crypto/x509"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"

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
	RegistryAuths   map[string]*RegistryAuthConfig   `yaml:"auths,omitempty" protobuf:"2"`
	RegistryTLSs    map[string]*RegistryTLSConfig    `yaml:"tls,omitempty" protobuf:"3"`
}

// Mirrors implements the Registries interface.
func (r RegistriesConfigSpec) Mirrors() map[string]config2.RegistryMirrorConfig {
	mirrors := make(map[string]config2.RegistryMirrorConfig, len(r.RegistryMirrors))

	for k, v := range r.RegistryMirrors {
		mirrors[k] = v
	}

	return mirrors
}

// Auths implements the Registries interface.
func (r RegistriesConfigSpec) Auths() map[string]config2.RegistryAuthConfig {
	auths := make(map[string]config2.RegistryAuthConfig, len(r.RegistryAuths))

	for k, v := range r.RegistryAuths {
		auths[k] = v
	}

	return auths
}

// TLSs implements the Registries interface.
func (r RegistriesConfigSpec) TLSs() map[string]RegistryTLSConfigExtended {
	tlss := make(map[string]RegistryTLSConfigExtended, len(r.RegistryTLSs))

	for k, v := range r.RegistryTLSs {
		tlss[k] = v
	}

	return tlss
}

// RegistryMirrorConfig represents mirror configuration for a registry.
//
//gotagsrewrite:gen
type RegistryMirrorConfig struct {
	MirrorEndpoints    []RegistryEndpointConfig `yaml:"endpoints" protobuf:"1"`
	MirrorSkipFallback bool                     `yaml:"skipFallback,omitempty" protobuf:"3"`
}

// SkipFallback implements the Registries interface.
func (r *RegistryMirrorConfig) SkipFallback() bool {
	return r.MirrorSkipFallback
}

// Endpoints implements the Registries interface.
func (r *RegistryMirrorConfig) Endpoints() []config2.RegistryEndpointConfig {
	return xslices.Map(r.MirrorEndpoints, func(endpoint RegistryEndpointConfig) config2.RegistryEndpointConfig {
		return endpoint
	})
}

// RegistryEndpointConfig represents a single registry endpoint.
//
//gotagsrewrite:gen
type RegistryEndpointConfig struct {
	EndpointEndpoint     string `yaml:"endpoint" protobuf:"1"`
	EndpointOverridePath bool   `yaml:"overridePath" protobuf:"2"`
}

// Endpoint implements the Registries interface.
func (r RegistryEndpointConfig) Endpoint() string {
	return r.EndpointEndpoint
}

// OverridePath implements the Registries interface.
func (r RegistryEndpointConfig) OverridePath() bool {
	return r.EndpointOverridePath
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

// Username implements the Registries interface.
func (r *RegistryAuthConfig) Username() string {
	return r.RegistryUsername
}

// Password implements the Registries interface.
func (r *RegistryAuthConfig) Password() string {
	return r.RegistryPassword
}

// Auth implements the Registries interface.
func (r *RegistryAuthConfig) Auth() string {
	return r.RegistryAuth
}

// IdentityToken implements the Registries interface.
func (r *RegistryAuthConfig) IdentityToken() string {
	return r.RegistryIdentityToken
}

// RegistryTLSConfig specifies TLS config for HTTPS registries.
//
//gotagsrewrite:gen
type RegistryTLSConfig struct {
	TLSClientIdentity     *x509.PEMEncodedCertificateAndKey `yaml:"clientIdentity,omitempty" protobuf:"1"`
	TLSCA                 v1alpha1.Base64Bytes              `yaml:"ca,omitempty" protobuf:"2"`
	TLSInsecureSkipVerify bool                              `yaml:"insecureSkipVerify,omitempty" protobuf:"3"`
}

// ClientIdentity implements the Registries interface.
func (r *RegistryTLSConfig) ClientIdentity() *x509.PEMEncodedCertificateAndKey {
	return r.TLSClientIdentity
}

// CA implements the Registries interface.
func (r *RegistryTLSConfig) CA() []byte {
	return r.TLSCA
}

// InsecureSkipVerify implements the Registries interface.
func (r *RegistryTLSConfig) InsecureSkipVerify() bool {
	return r.TLSInsecureSkipVerify
}

// GetTLSConfig prepares TLS configuration for connection.
func (r *RegistryTLSConfig) GetTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	if r.TLSClientIdentity != nil {
		cert, err := tls.X509KeyPair(r.TLSClientIdentity.Crt, r.TLSClientIdentity.Key)
		if err != nil {
			return nil, fmt.Errorf("error parsing client identity: %w", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if r.CA() != nil {
		tlsConfig.RootCAs = stdx509.NewCertPool()
		tlsConfig.RootCAs.AppendCertsFromPEM(r.TLSCA)
	}

	if r.InsecureSkipVerify() {
		tlsConfig.InsecureSkipVerify = true
	}

	return tlsConfig, nil
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

	err := protobuf.RegisterDynamic(RegistriesConfigType, &RegistriesConfig{})
	if err != nil {
		panic(err)
	}
}
