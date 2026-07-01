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

// KubeAggregatorCAConfig defines the KubeAggregatorCAConfig configuration name.
const KubeAggregatorCAConfig = "KubeAggregatorCAConfig"

func init() {
	registry.Register(KubeAggregatorCAConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeAggregatorCAConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sAggregatorCAConfig        = &KubeAggregatorCAConfigV1Alpha1{}
	_ config.Validator                    = &KubeAggregatorCAConfigV1Alpha1{}
	_ config.SecretDocument               = &KubeAggregatorCAConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeAggregatorCAConfigV1Alpha1{}
)

// KubeAggregatorCAConfigV1Alpha1 configures Kubernetes API aggregator accepted CAs.
//
//	examples:
//	  - value: exampleKubeAggregatorCAConfigV1Alpha1()
//	alias: KubeAggregatorCAConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeAggregatorCAConfig
type KubeAggregatorCAConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The currently active issuing certificate authority for the Kubernetes API aggregator flow.
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
	AggregatorIssuingCA *meta.CertificateAndKey `yaml:"issuingCA,omitempty"`
	//   description: |
	//     The list of accepted CA certificates for the Kubernetes API server aggregator flow.
	//
	//     This field should only be set for the controlplane machines.
	//     The value should be a PEM encoded certificate.
	//     The issuing CA certificate is automatically added to the list of accepted CAs.
	AggregatorAcceptedCAs []string `yaml:"acceptedCAs,omitempty"`
}

// NewKubeAggregatorCAConfigV1Alpha1 creates a new KubeAggregatorCAConfig config document.
func NewKubeAggregatorCAConfigV1Alpha1() *KubeAggregatorCAConfigV1Alpha1 {
	return &KubeAggregatorCAConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeAggregatorCAConfig,
		},
	}
}

func exampleKubeAggregatorCAConfigV1Alpha1() *KubeAggregatorCAConfigV1Alpha1 {
	cfg := NewKubeAggregatorCAConfigV1Alpha1()
	cfg.AggregatorIssuingCA = &meta.CertificateAndKey{
		Cert: "--- EXAMPLE CERTIFICATE ---",
		Key:  "--- EXAMPLE KEY ---",
	}
	cfg.AggregatorAcceptedCAs = []string{"--- EXAMPLE AGGREGATOR CA ---"}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeAggregatorCAConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:dupl
func (s *KubeAggregatorCAConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.AggregatorIssuingCA == nil {
		errs = errors.Join(errs, errors.New("issuing CA is not set"))
	} else if err := s.AggregatorIssuingCA.Validate(true); err != nil {
		errs = errors.Join(errs, fmt.Errorf("issuing CA: %w", err))
	}

	for idx, ca := range s.AggregatorAcceptedCAs {
		if err := meta.AssertValidPEM([]byte(ca), meta.PEMTypeCertificate); err != nil {
			errs = errors.Join(errs, fmt.Errorf("accepted CA #%d: %w", idx, err))
		}
	}

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeAggregatorCAConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil {
		if v1alpha1Cfg.ClusterConfig.ClusterAggregatorCA != nil { //nolint:staticcheck // testing deprecated field
			return errors.New("kube-apiserver aggregator CA is already set in v1alpha1 config (.cluster.aggregatorCA), please remove it and use the new KubeAggregatorCAConfig document instead")
		}
	}

	return nil
}

// K8sAggregatorCAConfigSignal implements config.K8sAggregatorCAConfig interface.
func (s *KubeAggregatorCAConfigV1Alpha1) K8sAggregatorCAConfigSignal() {}

// IssuingCA implements config.K8sAPIServerCAConfig interface.
func (s *KubeAggregatorCAConfigV1Alpha1) IssuingCA() *x509.PEMEncodedCertificateAndKey {
	return s.AggregatorIssuingCA.ToX509()
}

// AcceptedCAs implements config.K8sAPIServerCAConfig interface.
func (s *KubeAggregatorCAConfigV1Alpha1) AcceptedCAs() []*x509.PEMEncodedCertificate {
	result := xslices.Map(s.AggregatorAcceptedCAs, func(ca string) *x509.PEMEncodedCertificate {
		return &x509.PEMEncodedCertificate{
			Crt: []byte(ca),
		}
	})

	if s.AggregatorIssuingCA != nil {
		result = slices.Insert(result, 0, &x509.PEMEncodedCertificate{
			Crt: []byte(s.AggregatorIssuingCA.Cert),
		})
	}

	return result
}

// Redact implements config.SecretDocument interface.
func (s *KubeAggregatorCAConfigV1Alpha1) Redact(replacement string) {
	if s.AggregatorIssuingCA != nil {
		s.AggregatorIssuingCA.Key = replacement
	}
}
