// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	stdx509 "crypto/x509"
	"errors"
	"fmt"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// RegistryTLSConfig defines the RegistryTLSConfig configuration name.
const RegistryTLSConfig = "RegistryTLSConfig"

func init() {
	registry.Register(RegistryTLSConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &RegistryTLSConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.RegistryTLSConfigDocument = &RegistryTLSConfigV1Alpha1{}
	_ config.Validator                 = &RegistryTLSConfigV1Alpha1{}
	_ config.SecretDocument            = &RegistryTLSConfigV1Alpha1{}
	_ config.NamedDocument             = &RegistryTLSConfigV1Alpha1{}
)

// RegistryTLSConfigV1Alpha1 configures TLS for a registry endpoint.
//
//	examples:
//	  - value: exampleRegistryTLSConfigVAlpha1()
//	alias: RegistryTLSConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/RegistryTLSConfig
type RegistryTLSConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Registry endpoint to apply the TLS configuration to.
	//
	//     Registry endpoint is the hostname part of the endpoint URL,
	//     e.g. 'my-mirror.local:5000' for 'https://my-mirror.local:5000/v2/'.
	//
	//     The TLS configuration makes sense only for HTTPS endpoints.
	//     The TLS configuration will apply to all image pulls for this
	//     registry endpoint, by Talos or any Kubernetes workloads.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Enable mutual TLS authentication with the registry.
	//     Client certificate and key should be PEM-encoded.
	//   examples:
	//     - value: exampleCertificateAndKey()
	//   schema:
	//     type: object
	//     additionalProperties: false
	//     properties:
	//       crt:
	//         type: string
	//       key:
	//         type: string
	TLSClientIdentity *meta.CertificateAndKey `yaml:"clientIdentity,omitempty"`
	//   description: |
	//     CA registry certificate to add the list of trusted certificates.
	//     Certificate should be PEM-encoded.
	//   schema:
	//     type: string
	TLSCA string `yaml:"ca,omitempty"`
	//   description: |
	//     Skip TLS server certificate verification (not recommended).
	TLSInsecureSkipVerify *bool `yaml:"insecureSkipVerify,omitempty"`
}

// NewRegistryTLSConfigV1Alpha1 creates a new RegistryTLSConfig config document.
func NewRegistryTLSConfigV1Alpha1(name string) *RegistryTLSConfigV1Alpha1 {
	return &RegistryTLSConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       RegistryTLSConfig,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleRegistryTLSConfigVAlpha1() *RegistryTLSConfigV1Alpha1 {
	cfg := NewRegistryTLSConfigV1Alpha1("my-private-registry.local:5000")
	cfg.TLSCA = "-----BEGIN CERTIFICATE-----\nMIID...IDAQAB\n-----END CERTIFICATE-----"

	return cfg
}

func exampleCertificateAndKey() *meta.CertificateAndKey {
	return &meta.CertificateAndKey{
		Cert: "-----BEGIN CERTIFICATE-----\nMIID...IDAQAB\n-----END CERTIFICATE-----",
		Key:  "-----BEGIN PRIVATE KEY-----\nMIIE...AB\n-----END PRIVATE KEY-----",
	}
}

// Clone implements config.Document interface.
func (s *RegistryTLSConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.Document interface.
func (s *RegistryTLSConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Validate implements config.Validator interface.
func (s *RegistryTLSConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if s.TLSCA != "" {
		if !stdx509.NewCertPool().AppendCertsFromPEM([]byte(s.TLSCA)) {
			errs = errors.Join(errs, errors.New("ca must be a valid PEM-encoded certificate"))
		}
	}

	if s.TLSClientIdentity != nil {
		keyPair := s.ClientIdentity()

		_, err := keyPair.GetCert()
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("client identity certificate is invalid: %w", err))
		}

		_, err = keyPair.GetPrivateKey()
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("client identity key is invalid: %w", err))
		}
	}

	return warnings, errs
}

// ClientIdentity implements config.RegistryTLSConfigDocument interface.
func (s *RegistryTLSConfigV1Alpha1) ClientIdentity() *x509.PEMEncodedCertificateAndKey {
	if s.TLSClientIdentity == nil {
		return nil
	}

	return &x509.PEMEncodedCertificateAndKey{
		Crt: []byte(s.TLSClientIdentity.Cert),
		Key: []byte(s.TLSClientIdentity.Key),
	}
}

// CA implements config.RegistryTLSConfigDocument interface.
func (s *RegistryTLSConfigV1Alpha1) CA() []byte {
	if s.TLSCA == "" {
		return nil
	}

	return []byte(s.TLSCA)
}

// InsecureSkipVerify implements config.RegistryTLSConfigDocument interface.
func (s *RegistryTLSConfigV1Alpha1) InsecureSkipVerify() bool {
	return pointer.SafeDeref(s.TLSInsecureSkipVerify)
}

// Redact implements config.SecretDocument interface.
func (s *RegistryTLSConfigV1Alpha1) Redact(replacement string) {
	if s.TLSClientIdentity != nil {
		if s.TLSClientIdentity.Key != "" {
			s.TLSClientIdentity.Key = replacement
		}
	}

	if s.TLSCA != "" {
		s.TLSCA = replacement
	}
}
