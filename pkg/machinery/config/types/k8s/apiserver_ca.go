// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

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

// KubeAPIServerCAConfig defines the KubeAPIServerCAConfig configuration name.
const KubeAPIServerCAConfig = "KubeAPIServerCAConfig"

func init() {
	registry.Register(KubeAPIServerCAConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeAPIServerCAConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sAPIServerCAConfig         = &KubeAPIServerCAConfigV1Alpha1{}
	_ config.Validator                    = &KubeAPIServerCAConfigV1Alpha1{}
	_ config.SecretDocument               = &KubeAPIServerCAConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeAPIServerCAConfigV1Alpha1{}
)

// KubeAPIServerCAConfigV1Alpha1 configures Kubernetes API server CA.
//
//	examples:
//	  - value: exampleKubeAPIServerCAConfigV1Alpha1()
//	alias: KubeAPIServerCAConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeAPIServerCAConfig
type KubeAPIServerCAConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The currently active issuing certificate authority for the Kubernetes API server.
	//
	//     This field should only be set for the controlplane machines.
	//     The value contains a private key and a certificate, PEM encoded.
	//   schema:
	//     type: object
	//     additionalProperties: false
	//     properties:
	//       cert:
	//         type: string
	//       key:
	//         type: string
	APIIssuingCA *meta.CertificateAndKey `yaml:"issuingCA,omitempty"`
	//   description: |
	//     The list of accepted CA certificates for the Kubernetes API server.
	//
	//     The value should be a PEM encoded certificate.
	//     The issuing CA certificate is automatically added to the list of accepted CAs.
	APIAcceptedCAs []string `yaml:"acceptedCAs,omitempty"`
}

// NewKubeAPIServerCAConfigV1Alpha1 creates a new KubeAPIServerCAConfig config document.
func NewKubeAPIServerCAConfigV1Alpha1() *KubeAPIServerCAConfigV1Alpha1 {
	return &KubeAPIServerCAConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeAPIServerCAConfig,
		},
	}
}

func exampleKubeAPIServerCAConfigV1Alpha1() *KubeAPIServerCAConfigV1Alpha1 {
	cfg := NewKubeAPIServerCAConfigV1Alpha1()
	cfg.APIIssuingCA = &meta.CertificateAndKey{
		Cert: "--- EXAMPLE CERTIFICATE ---",
		Key:  "--- EXAMPLE KEY ---",
	}
	cfg.APIAcceptedCAs = []string{"--- EXAMPLE CA ---"}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeAPIServerCAConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:dupl
func (s *KubeAPIServerCAConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.APIIssuingCA != nil {
		if err := s.APIIssuingCA.Validate(true); err != nil {
			errs = errors.Join(errs, fmt.Errorf("issuing CA: %w", err))
		}
	}

	for idx, ca := range s.APIAcceptedCAs {
		if err := meta.AssertValidPEM([]byte(ca), meta.PEMTypeCertificate); err != nil {
			errs = errors.Join(errs, fmt.Errorf("accepted CA #%d: %w", idx, err))
		}
	}

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeAPIServerCAConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil {
		if v1alpha1Cfg.ClusterConfig.ClusterCA != nil { //nolint:staticcheck // testing deprecated field
			return errors.New("kube-apiserver CA is already set in v1alpha1 config (.cluster.ca)")
		}
	}

	return nil
}

// K8sAPIServerCAConfigSignal implements config.K8sAPIServerCAConfig interface.
func (s *KubeAPIServerCAConfigV1Alpha1) K8sAPIServerCAConfigSignal() {}

// IssuingCA implements config.K8sAPIServerCAConfig interface.
func (s *KubeAPIServerCAConfigV1Alpha1) IssuingCA() *x509.PEMEncodedCertificateAndKey {
	return s.APIIssuingCA.ToX509()
}

// AcceptedCAs implements config.K8sAPIServerCAConfig interface.
func (s *KubeAPIServerCAConfigV1Alpha1) AcceptedCAs() []*x509.PEMEncodedCertificate {
	result := xslices.Map(s.APIAcceptedCAs, func(ca string) *x509.PEMEncodedCertificate {
		return &x509.PEMEncodedCertificate{
			Crt: []byte(ca),
		}
	})

	if s.APIIssuingCA != nil {
		result = slices.Insert(result, 0, &x509.PEMEncodedCertificate{
			Crt: []byte(s.APIIssuingCA.Cert),
		})
	}

	return result
}

// Redact implements config.SecretDocument interface.
func (s *KubeAPIServerCAConfigV1Alpha1) Redact(replacement string) {
	if s.APIIssuingCA != nil {
		s.APIIssuingCA.Key = replacement
	}
}
