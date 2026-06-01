// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"errors"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// RegistryAuthConfig defines the RegistryAuthConfig configuration name.
const RegistryAuthConfig = "RegistryAuthConfig"

func init() {
	registry.Register(RegistryAuthConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &RegistryAuthConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.RegistryAuthConfigDocument = &RegistryAuthConfigV1Alpha1{}
	_ config.Validator                  = &RegistryAuthConfigV1Alpha1{}
	_ config.SecretDocument             = &RegistryAuthConfigV1Alpha1{}
	_ config.NamedDocument              = &RegistryAuthConfigV1Alpha1{}
)

// RegistryAuthConfigV1Alpha1 configures authentication for a registry endpoint.
//
//	examples:
//	  - value: exampleRegistryAuthConfigVAlpha1()
//	alias: RegistryAuthConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/RegistryAuthConfig
type RegistryAuthConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Registry endpoint to apply the authentication configuration to.
	//
	//     Registry endpoint is the hostname part of the endpoint URL,
	//     e.g. 'my-mirror.local:5000' for 'https://my-mirror.local:5000/v2/'.
	//
	//     The authentication configuration will apply to all image pulls for this
	//     registry endpoint, by Talos or any Kubernetes workloads.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Username/password authentication.
	RegistryUsername string `yaml:"username,omitempty"`
	//   description: |
	//     Username/password authentication.
	RegistryPassword string `yaml:"password,omitempty"`
	//   description: |
	//     Raw authentication string.
	RegistryAuth string `yaml:"auth,omitempty"`
	//   description: |
	//     Identity token authentication.
	RegistryIdentityToken string `yaml:"identityToken,omitempty"`
}

// NewRegistryAuthConfigV1Alpha1 creates a new RegistryAuthConfig config document.
func NewRegistryAuthConfigV1Alpha1(name string) *RegistryAuthConfigV1Alpha1 {
	return &RegistryAuthConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       RegistryAuthConfig,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleRegistryAuthConfigVAlpha1() *RegistryAuthConfigV1Alpha1 {
	cfg := NewRegistryAuthConfigV1Alpha1("my-private-registry.local:5000")
	cfg.RegistryUsername = "my-username"
	cfg.RegistryPassword = "my-password"

	return cfg
}

// Clone implements config.Document interface.
func (s *RegistryAuthConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.Document interface.
func (s *RegistryAuthConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Validate implements config.Validator interface.
func (s *RegistryAuthConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	hasUsername := s.RegistryUsername != ""
	hasIdentityToken := s.RegistryIdentityToken != ""
	hasAuth := s.RegistryAuth != ""

	if hasUsername && hasIdentityToken {
		errs = errors.Join(errs, errors.New("only one of username/password or identityToken authentication can be specified"))
	}

	if hasAuth && (hasUsername || hasIdentityToken) {
		errs = errors.Join(errs, errors.New("only one of auth, username/password or identityToken authentication can be specified"))
	}

	return warnings, errs
}

// Username implements config.RegistryAuthConfig interface.
func (s *RegistryAuthConfigV1Alpha1) Username() string {
	return s.RegistryUsername
}

// Password implements config.RegistryAuthConfig interface.
func (s *RegistryAuthConfigV1Alpha1) Password() string {
	return s.RegistryPassword
}

// Auth implements config.RegistryAuthConfig interface.
func (s *RegistryAuthConfigV1Alpha1) Auth() string {
	return s.RegistryAuth
}

// IdentityToken implements config.RegistryAuthConfig interface.
func (s *RegistryAuthConfigV1Alpha1) IdentityToken() string {
	return s.RegistryIdentityToken
}

// Redact implements config.SecretDocument interface.
func (s *RegistryAuthConfigV1Alpha1) Redact(replacement string) {
	if s.RegistryPassword != "" {
		s.RegistryPassword = replacement
	}

	if s.RegistryAuth != "" {
		s.RegistryAuth = replacement
	}

	if s.RegistryIdentityToken != "" {
		s.RegistryIdentityToken = replacement
	}
}
