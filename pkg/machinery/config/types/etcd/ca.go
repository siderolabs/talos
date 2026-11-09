// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"errors"
	"fmt"
	"slices"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// EtcdCAConfig defines the EtcdCAConfig configuration name.
const EtcdCAConfig = "EtcdCAConfig"

func init() {
	registry.Register(EtcdCAConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &EtcdCAConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.EtcdCAConfig                 = &EtcdCAConfigV1Alpha1{}
	_ config.Validator                    = &EtcdCAConfigV1Alpha1{}
	_ config.SecretDocument               = &EtcdCAConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &EtcdCAConfigV1Alpha1{}
	_ container.ControlplaneOnlyConfig    = &EtcdCAConfigV1Alpha1{}
)

// EtcdCAConfigV1Alpha1 configures etcd PKI (certificate authority).
//
//	description: |
//	  The etcd CA is used to issue the certificates for the etcd members (server and peer),
//	  and the etcd clients (e.g. kube-apiserver, Talos itself).
//
//	  The accepted CAs are trusted by the etcd members (for client and peer connections),
//	  and by the etcd clients (for server connections), in addition to the issuing CA.
//
//	  This document should only be present on the controlplane machines.
//	examples:
//	  - value: exampleEtcdCAConfigV1Alpha1()
//	alias: EtcdCAConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/EtcdCAConfig
type EtcdCAConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The issuing certificate authority for etcd.
	//
	//     The value contains a private key and a certificate, PEM encoded.
	//   schemaRequired: true
	//   schema:
	//     type: object
	//     additionalProperties: false
	//     properties:
	//       cert:
	//         type: string
	//       key:
	//         type: string
	CAIssuingCA *meta.CertificateAndKey `yaml:"issuingCA"`
	//   description: |
	//     The list of additional accepted CA certificates for etcd.
	//
	//     The value should be a PEM encoded certificate.
	//     The issuing CA certificate is automatically added to the list of accepted CAs.
	CAAcceptedCAs []string `yaml:"acceptedCAs,omitempty"`
}

// NewEtcdCAConfigV1Alpha1 creates a new EtcdCAConfig config document.
func NewEtcdCAConfigV1Alpha1() *EtcdCAConfigV1Alpha1 {
	return &EtcdCAConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       EtcdCAConfig,
		},
	}
}

func exampleEtcdCAConfigV1Alpha1() *EtcdCAConfigV1Alpha1 {
	cfg := NewEtcdCAConfigV1Alpha1()
	cfg.CAIssuingCA = &meta.CertificateAndKey{
		Cert: "--- EXAMPLE CERTIFICATE ---",
		Key:  "--- EXAMPLE KEY ---",
	}
	cfg.CAAcceptedCAs = []string{"--- EXAMPLE CA ---"}

	return cfg
}

// Clone implements config.Document interface.
func (s *EtcdCAConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *EtcdCAConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if s.CAIssuingCA == nil {
		errs = errors.Join(errs, errors.New("issuing CA is required"))
	} else if err := s.CAIssuingCA.Validate(true); err != nil {
		errs = errors.Join(errs, fmt.Errorf("issuing CA: %w", err))
	}

	for idx, ca := range s.CAAcceptedCAs {
		if err := meta.AssertValidPEM([]byte(ca), meta.PEMTypeCertificate); err != nil {
			errs = errors.Join(errs, fmt.Errorf("accepted CA #%d: %w", idx, err))
		}
	}

	return nil, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *EtcdCAConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	return v1alpha1ConflictValidate(v1alpha1Cfg)
}

// ControlplaneOnlyDocument implements container.ControlplaneOnlyConfig interface.
func (s *EtcdCAConfigV1Alpha1) ControlplaneOnlyDocument() {}

// EtcdCAConfigSignal implements config.EtcdCAConfig interface.
func (s *EtcdCAConfigV1Alpha1) EtcdCAConfigSignal() {}

// IssuingCA implements config.EtcdCAConfig interface.
func (s *EtcdCAConfigV1Alpha1) IssuingCA() *x509.PEMEncodedCertificateAndKey {
	return s.CAIssuingCA.ToX509()
}

// AcceptedCAs implements config.EtcdCAConfig interface.
func (s *EtcdCAConfigV1Alpha1) AcceptedCAs() []*x509.PEMEncodedCertificate {
	result := xslices.Map(s.CAAcceptedCAs, func(ca string) *x509.PEMEncodedCertificate {
		return &x509.PEMEncodedCertificate{
			Crt: []byte(ca),
		}
	})

	if s.CAIssuingCA != nil {
		result = slices.Insert(result, 0, &x509.PEMEncodedCertificate{
			Crt: []byte(s.CAIssuingCA.Cert),
		})
	}

	return result
}

// Redact implements config.SecretDocument interface.
func (s *EtcdCAConfigV1Alpha1) Redact(replacement string) {
	if s.CAIssuingCA != nil {
		s.CAIssuingCA.Key = replacement
	}
}

func v1alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.EtcdConfig != nil { //nolint:staticcheck // testing deprecated field
		return errors.New("etcd config is already set in v1alpha1 config (.cluster.etcd)")
	}

	return nil
}
