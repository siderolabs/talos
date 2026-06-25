// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	_ "embed"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/aggregatorcaconfig.yaml
var expectedKubeAggregatorCAConfigDocument []byte

func TestKubeAggregatorCAConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAggregatorCAConfigV1Alpha1()
	cfg.AggregatorIssuingCA = &meta.CertificateAndKey{
		Cert: "-----BEGIN CERTIFICATE-----\nMIIC0DCCAbigAwIBAgIUI7z\n-----END CERTIFICATE-----",
		Key:  "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIABC\n-----END EC PRIVATE KEY-----",
	}
	cfg.AggregatorAcceptedCAs = []string{
		"-----BEGIN CERTIFICATE-----\nMIIACCEPTED1\n-----END CERTIFICATE-----",
		"-----BEGIN CERTIFICATE-----\nMIIACCEPTED2\n-----END CERTIFICATE-----",
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeAggregatorCAConfigDocument, marshaled)
}

func TestKubeAggregatorCAConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeAggregatorCAConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeAggregatorCAConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeAggregatorCAConfig,
		},
		AggregatorIssuingCA: &meta.CertificateAndKey{
			Cert: "-----BEGIN CERTIFICATE-----\nMIIC0DCCAbigAwIBAgIUI7z\n-----END CERTIFICATE-----",
			Key:  "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIABC\n-----END EC PRIVATE KEY-----",
		},
		AggregatorAcceptedCAs: []string{
			"-----BEGIN CERTIFICATE-----\nMIIACCEPTED1\n-----END CERTIFICATE-----",
			"-----BEGIN CERTIFICATE-----\nMIIACCEPTED2\n-----END CERTIFICATE-----",
		},
	}, docs[0])
}

func TestKubeAggregatorCAConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeAggregatorCAConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeAggregatorCAConfigV1Alpha1,

			expectedError: "issuing CA is not set",
		},
		{
			name: "valid",
			cfg: func() *k8s.KubeAggregatorCAConfigV1Alpha1 {
				ca, err := x509.NewSelfSignedCertificateAuthority()
				require.NoError(t, err)

				cfg := k8s.NewKubeAggregatorCAConfigV1Alpha1()
				cfg.AggregatorIssuingCA = &meta.CertificateAndKey{
					Cert: string(ca.CrtPEM),
					Key:  string(ca.KeyPEM),
				}
				cfg.AggregatorAcceptedCAs = []string{string(ca.CrtPEM)}

				return cfg
			},
		},
		{
			name: "invalid issuing CA",
			cfg: func() *k8s.KubeAggregatorCAConfigV1Alpha1 {
				cfg := k8s.NewKubeAggregatorCAConfigV1Alpha1()
				cfg.AggregatorIssuingCA = &meta.CertificateAndKey{
					Cert: "not a PEM",
					Key:  "not a PEM",
				}

				return cfg
			},

			expectedError: "issuing CA: certificate: no PEM blocks found",
		},
		{
			name: "invalid accepted CA",
			cfg: func() *k8s.KubeAggregatorCAConfigV1Alpha1 {
				ca, err := x509.NewSelfSignedCertificateAuthority()
				require.NoError(t, err)

				cfg := k8s.NewKubeAggregatorCAConfigV1Alpha1()
				cfg.AggregatorIssuingCA = &meta.CertificateAndKey{
					Cert: string(ca.CrtPEM),
					Key:  string(ca.KeyPEM),
				}
				cfg.AggregatorAcceptedCAs = []string{"not a PEM"}

				return cfg
			},

			expectedError: "accepted CA #0: no PEM blocks found",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

//nolint:dupl
func TestKubeAggregatorCAConfigAcceptedCAs(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAggregatorCAConfigV1Alpha1()
	cfg.AggregatorIssuingCA = &meta.CertificateAndKey{
		Cert: "ISSUING",
		Key:  "ISSUING-KEY",
	}
	cfg.AggregatorAcceptedCAs = []string{"ACCEPTED1", "ACCEPTED2"}

	accepted := cfg.AcceptedCAs()

	require.Len(t, accepted, 3)

	// the issuing CA certificate is prepended to the list of accepted CAs
	assert.Equal(t, "ISSUING", string(accepted[0].Crt))
	assert.Equal(t, "ACCEPTED1", string(accepted[1].Crt))
	assert.Equal(t, "ACCEPTED2", string(accepted[2].Crt))
}

func TestKubeAggregatorCAConfigAcceptedCAsNoIssuingCA(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAggregatorCAConfigV1Alpha1()
	cfg.AggregatorAcceptedCAs = []string{"ACCEPTED1"}

	accepted := cfg.AcceptedCAs()

	require.Len(t, accepted, 1)
	assert.Equal(t, "ACCEPTED1", string(accepted[0].Crt))
}

func TestKubeAggregatorCAConfigIssuingCA(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAggregatorCAConfigV1Alpha1()
	cfg.AggregatorIssuingCA = &meta.CertificateAndKey{
		Cert: "ISSUING",
		Key:  "ISSUING-KEY",
	}

	issuing := cfg.IssuingCA()
	require.NotNil(t, issuing)
	assert.Equal(t, "ISSUING", string(issuing.Crt))
	assert.Equal(t, "ISSUING-KEY", string(issuing.Key))
}

//go:embed testdata/aggregatorcaconfig_redacted.yaml
var expectedKubeAggregatorCAConfigRedactedDocument []byte

func TestKubeAggregatorCAConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAggregatorCAConfigV1Alpha1()
	cfg.AggregatorIssuingCA = &meta.CertificateAndKey{
		Cert: "-----BEGIN CERTIFICATE-----\nMIIC0DCCAbigAwIBAgIUI7z\n-----END CERTIFICATE-----",
		Key:  "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIABC\n-----END EC PRIVATE KEY-----",
	}
	cfg.AggregatorAcceptedCAs = []string{
		"-----BEGIN CERTIFICATE-----\nMIIACCEPTED1\n-----END CERTIFICATE-----",
	}

	cfg.Redact("REDACTED")

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeAggregatorCAConfigRedactedDocument, marshaled)
}

//nolint:dupl
func TestKubeAggregatorCAConfigV1Alpha1Validate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		v1alpha1Cfg *v1alpha1.Config

		expectedError string
	}{
		{
			name:        "empty",
			v1alpha1Cfg: &v1alpha1.Config{},
		},
		{
			name: "v1alpha1 with aggregator CA set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterAggregatorCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
			},

			expectedError: "kube-apiserver aggregator CA is already set in v1alpha1 config (.cluster.aggregatorCA), please remove it and use the new KubeAggregatorCAConfig document instead",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeAggregatorCAConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
