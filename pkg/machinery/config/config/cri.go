// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"github.com/siderolabs/crypto/x509"
)

// RegistryMirrorConfigDocument is registry mirror configuration document.
type RegistryMirrorConfigDocument interface {
	NamedDocument
	RegistryMirrorConfig
}

// RegistryAuthConfigDocument is registry authentication configuration document.
type RegistryAuthConfigDocument interface {
	NamedDocument
	RegistryAuthConfig
}

// RegistryTLSConfigDocument is registry TLS configuration document.
type RegistryTLSConfigDocument interface {
	NamedDocument
	RegistryTLSConfig
}

// RegistryMirrorConfig represents mirror configuration for a registry.
type RegistryMirrorConfig interface {
	Endpoints() []RegistryEndpointConfig
	SkipFallback() bool
}

// RegistryEndpointConfig represents a single registry endpoint.
type RegistryEndpointConfig interface {
	Endpoint() string
	OverridePath() bool
}

// RegistryAuthConfig specifies authentication configuration for a registry.
type RegistryAuthConfig interface {
	Username() string
	Password() string
	Auth() string
	IdentityToken() string
}

// RegistryTLSConfig specifies TLS config for HTTPS registries.
type RegistryTLSConfig interface {
	ClientIdentity() *x509.PEMEncodedCertificateAndKey
	CA() []byte
	InsecureSkipVerify() bool
}
