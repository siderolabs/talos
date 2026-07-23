// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"net/url"
	"slices"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// KubeServiceAccountConfig defines the KubeServiceAccountConfig configuration name.
const KubeServiceAccountConfig = "KubeServiceAccountConfig"

func init() {
	registry.Register(KubeServiceAccountConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeServiceAccountConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sServiceAccountConfig      = &KubeServiceAccountConfigV1Alpha1{}
	_ config.Validator                    = &KubeServiceAccountConfigV1Alpha1{}
	_ config.SecretDocument               = &KubeServiceAccountConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeServiceAccountConfigV1Alpha1{}
	_ container.ControlplaneOnlyConfig    = &KubeServiceAccountConfigV1Alpha1{}
)

// KubeServiceAccountConfigV1Alpha1 configures Kubernetes service accounts.
//
//	examples:
//	  - value: exampleKubeServiceAccountConfigV1Alpha1()
//	alias: KubeServiceAccountConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeServiceAccountConfig
type KubeServiceAccountConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The service account issuer configuration.
	//
	//     This configures how the service accounts are issued in Kubernetes.
	//   schemaRequired: true
	ServiceIssuer IssuerServiceAccountConfig `yaml:"issuer"`
	//   description: |
	//     The additional service accounts which are accepted by the Kubernetes API server.
	//
	//     This might be used for service account rotation, or for accepting service accounts from other clusters,
	//     or for accepting service accounts from other issuers.
	ServiceAccepted AcceptedServiceAccountConfig `yaml:"accepted,omitempty"`
}

// IssuerServiceAccountConfig configures the service account issuer.
type IssuerServiceAccountConfig struct {
	//   description: |
	//     The key which is used to sign the service account tokens.
	//
	//     This key is used to sign the service account tokens, and it is used by the Kubernetes API server to verify the service account tokens.
	//     The key must be a valid PEM encoded RSA or ECDSA private key.
	//   schemaRequired: true
	PrivateKey string `yaml:"privateKey"`
	//   description: |
	//     The issuer URL which is used to sign the service account tokens.
	//
	//     This URL is used to sign the service account tokens, and it is used by the Kubernetes API server to verify the service account tokens.
	//   schema:
	//     type: string
	//     pattern: "^(http|https)://"
	//   schemaRequired: true
	IssuerURL meta.URL `yaml:"issuerURL"`
}

// AcceptedServiceAccountConfig configures the accepted service accounts.
type AcceptedServiceAccountConfig struct {
	//   description: |
	//     The list of public keys which are used to verify the service account tokens.
	//
	//     These keys are used by the Kubernetes API server to verify the service account tokens.
	//     The keys must be valid PEM encoded RSA or ECDSA public keys.
	PublicKeys []string `yaml:"publicKeys,omitempty"`
	//   description: |
	//     The additional service account issuers which are accepted by the Kubernetes API server.
	//
	//     This might be used for service account rotation, or for accepting service accounts from other clusters,
	//     or for accepting service accounts from other issuers.
	//   schema:
	//     type: array
	//     items:
	//       type: string
	//       pattern: "^(http|https)://"
	Issuers []meta.URL `yaml:"issuers,omitempty"`
	//   description: |
	//     The list of API audiences for which the service account tokens are accepted by the Kubernetes API server.
	//
	//     If this field is not set, the default is to set to the issuer URL of the service account issuer.
	Audiences []string `yaml:"audiences,omitempty"`
}

// NewKubeServiceAccountConfigV1Alpha1 creates a new KubeServiceAccountConfig config document.
func NewKubeServiceAccountConfigV1Alpha1() *KubeServiceAccountConfigV1Alpha1 {
	return &KubeServiceAccountConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeServiceAccountConfig,
		},
	}
}

func exampleKubeServiceAccountConfigV1Alpha1() *KubeServiceAccountConfigV1Alpha1 {
	cfg := NewKubeServiceAccountConfigV1Alpha1()
	cfg.ServiceIssuer = IssuerServiceAccountConfig{
		PrivateKey: "--- EXAMPLE PRIVATE KEY ---",
		IssuerURL:  meta.URL{URL: ensure.Value(url.Parse("https://my-control-plane:6443"))},
	}
	cfg.ServiceAccepted = AcceptedServiceAccountConfig{
		PublicKeys: []string{"--- EXAMPLE PUBLIC KEY ---"},
		Issuers:    []meta.URL{{URL: ensure.Value(url.Parse("https://another-control-plane:6443"))}},
		Audiences:  []string{"https://another-control-plane:6443", "https://my-control-plane:6443"},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeServiceAccountConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:dupl
func (s *KubeServiceAccountConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	// the issuer section is all required
	if s.ServiceIssuer.PrivateKey == "" {
		errs = errors.Join(errs, errors.New("service issuer private key is required"))
	} else {
		key := x509.PEMEncodedKey{Key: []byte(s.ServiceIssuer.PrivateKey)}

		_, err := key.GetKey()
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("service issuer private key is invalid: %w", err))
		}
	}

	if s.ServiceIssuer.IssuerURL.URL == nil {
		errs = errors.Join(errs, errors.New("service issuer URL is required"))
	}

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeServiceAccountConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil {
		if v1alpha1Cfg.ClusterConfig.ClusterServiceAccount != nil { //nolint:staticcheck // testing deprecated field
			return errors.New("service account is already set in v1alpha1 config (.cluster.serviceAccount)")
		}
	}

	return nil
}

// K8sServiceAccountConfigSignal implements config.K8sServiceAccountConfig interface.
func (s *KubeServiceAccountConfigV1Alpha1) K8sServiceAccountConfigSignal() {}

// IssuingKey implements config.K8sServiceAccountConfig interface.
func (s *KubeServiceAccountConfigV1Alpha1) IssuingKey() *x509.PEMEncodedKey {
	return &x509.PEMEncodedKey{Key: []byte(s.ServiceIssuer.PrivateKey)}
}

// AcceptedKeys implements config.K8sServiceAccountConfig interface.
func (s *KubeServiceAccountConfigV1Alpha1) AcceptedKeys() []*x509.PEMEncodedKey {
	keys := xslices.Map(s.ServiceAccepted.PublicKeys, func(key string) *x509.PEMEncodedKey {
		return &x509.PEMEncodedKey{Key: []byte(key)}
	})

	// now, convert the issuing private key into a public key
	issuingKey, err := s.IssuingKey().GetKey()
	if err != nil {
		// the key is validated, so this should never be reached
		panic("unexpected error converting issuing private key to public key: " + err.Error())
	}

	keys = slices.Insert(keys, 0, &x509.PEMEncodedKey{Key: issuingKey.GetPublicKeyPEM()})

	return keys
}

// IssuerURL implements config.K8sServiceAccountConfig interface.
func (s *KubeServiceAccountConfigV1Alpha1) IssuerURL() string {
	return s.ServiceIssuer.IssuerURL.String()
}

// AcceptedIssuers implements config.K8sServiceAccountConfig interface.
func (s *KubeServiceAccountConfigV1Alpha1) AcceptedIssuers() []string {
	return xslices.Map(s.ServiceAccepted.Issuers, func(u meta.URL) string {
		return u.String()
	})
}

// APIAudiences implements config.K8sServiceAccountConfig interface.
func (s *KubeServiceAccountConfigV1Alpha1) APIAudiences() []string {
	if len(s.ServiceAccepted.Audiences) > 0 {
		return s.ServiceAccepted.Audiences
	}

	return []string{s.IssuerURL()}
}

// Redact implements config.SecretDocument interface.
func (s *KubeServiceAccountConfigV1Alpha1) Redact(replacement string) {
	if s.ServiceIssuer.PrivateKey != "" {
		s.ServiceIssuer.PrivateKey = replacement
	}
}

// ControlplaneOnlyDocument implements container.ControlplaneOnlyConfig interface.
func (s *KubeServiceAccountConfigV1Alpha1) ControlplaneOnlyDocument() {}
