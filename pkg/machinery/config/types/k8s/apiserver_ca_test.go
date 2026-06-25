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

//go:embed testdata/apiservercaconfig.yaml
var expectedKubeAPIServerCAConfigDocument []byte

func TestKubeAPIServerCAConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAPIServerCAConfigV1Alpha1()
	cfg.APIIssuingCA = &meta.CertificateAndKey{
		Cert: "-----BEGIN CERTIFICATE-----\nMIIC0DCCAbigAwIBAgIUI7z\n-----END CERTIFICATE-----",
		Key:  "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIABC\n-----END EC PRIVATE KEY-----",
	}
	cfg.APIAcceptedCAs = []string{
		"-----BEGIN CERTIFICATE-----\nMIIACCEPTED1\n-----END CERTIFICATE-----",
		"-----BEGIN CERTIFICATE-----\nMIIACCEPTED2\n-----END CERTIFICATE-----",
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeAPIServerCAConfigDocument, marshaled)
}

func TestKubeAPIServerCAConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeAPIServerCAConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeAPIServerCAConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeAPIServerCAConfig,
		},
		APIIssuingCA: &meta.CertificateAndKey{
			Cert: "-----BEGIN CERTIFICATE-----\nMIIC0DCCAbigAwIBAgIUI7z\n-----END CERTIFICATE-----",
			Key:  "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIABC\n-----END EC PRIVATE KEY-----",
		},
		APIAcceptedCAs: []string{
			"-----BEGIN CERTIFICATE-----\nMIIACCEPTED1\n-----END CERTIFICATE-----",
			"-----BEGIN CERTIFICATE-----\nMIIACCEPTED2\n-----END CERTIFICATE-----",
		},
	}, docs[0])
}

func TestKubeAPIServerCAConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeAPIServerCAConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeAPIServerCAConfigV1Alpha1,
		},
		{
			name: "valid",
			cfg: func() *k8s.KubeAPIServerCAConfigV1Alpha1 {
				ca, err := x509.NewSelfSignedCertificateAuthority()
				require.NoError(t, err)

				cfg := k8s.NewKubeAPIServerCAConfigV1Alpha1()
				cfg.APIIssuingCA = &meta.CertificateAndKey{
					Cert: string(ca.CrtPEM),
					Key:  string(ca.KeyPEM),
				}
				cfg.APIAcceptedCAs = []string{string(ca.CrtPEM)}

				return cfg
			},
		},
		{
			name: "invalid issuing CA",
			cfg: func() *k8s.KubeAPIServerCAConfigV1Alpha1 {
				cfg := k8s.NewKubeAPIServerCAConfigV1Alpha1()
				cfg.APIIssuingCA = &meta.CertificateAndKey{
					Cert: "not a PEM",
					Key:  "not a PEM",
				}

				return cfg
			},

			expectedError: "issuing CA: certificate: no PEM blocks found",
		},
		{
			name: "invalid accepted CA",
			cfg: func() *k8s.KubeAPIServerCAConfigV1Alpha1 {
				cfg := k8s.NewKubeAPIServerCAConfigV1Alpha1()
				cfg.APIAcceptedCAs = []string{"not a PEM"}

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
func TestKubeAPIServerCAConfigAcceptedCAs(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAPIServerCAConfigV1Alpha1()
	cfg.APIIssuingCA = &meta.CertificateAndKey{
		Cert: "ISSUING",
		Key:  "ISSUING-KEY",
	}
	cfg.APIAcceptedCAs = []string{"ACCEPTED1", "ACCEPTED2"}

	accepted := cfg.AcceptedCAs()

	require.Len(t, accepted, 3)

	// the issuing CA certificate is prepended to the list of accepted CAs
	assert.Equal(t, "ISSUING", string(accepted[0].Crt))
	assert.Equal(t, "ACCEPTED1", string(accepted[1].Crt))
	assert.Equal(t, "ACCEPTED2", string(accepted[2].Crt))
}

func TestKubeAPIServerCAConfigAcceptedCAsNoIssuingCA(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAPIServerCAConfigV1Alpha1()
	cfg.APIAcceptedCAs = []string{"ACCEPTED1"}

	accepted := cfg.AcceptedCAs()

	require.Len(t, accepted, 1)
	assert.Equal(t, "ACCEPTED1", string(accepted[0].Crt))
}

func TestKubeAPIServerCAConfigIssuingCA(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAPIServerCAConfigV1Alpha1()
	assert.Nil(t, cfg.IssuingCA())

	cfg.APIIssuingCA = &meta.CertificateAndKey{
		Cert: "ISSUING",
		Key:  "ISSUING-KEY",
	}

	issuing := cfg.IssuingCA()
	require.NotNil(t, issuing)
	assert.Equal(t, "ISSUING", string(issuing.Crt))
	assert.Equal(t, "ISSUING-KEY", string(issuing.Key))
}

//go:embed testdata/apiservercaconfig_redacted.yaml
var expectedKubeAPIServerCAConfigRedactedDocument []byte

func TestKubeAPIServerCAConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAPIServerCAConfigV1Alpha1()
	cfg.APIIssuingCA = &meta.CertificateAndKey{
		Cert: "-----BEGIN CERTIFICATE-----\nMIIC0DCCAbigAwIBAgIUI7z\n-----END CERTIFICATE-----",
		Key:  "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIABC\n-----END EC PRIVATE KEY-----",
	}
	cfg.APIAcceptedCAs = []string{
		"-----BEGIN CERTIFICATE-----\nMIIACCEPTED1\n-----END CERTIFICATE-----",
	}

	cfg.Redact("REDACTED")

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeAPIServerCAConfigRedactedDocument, marshaled)
}

//nolint:dupl
func TestKubeAPIServerCAConfigV1Alpha1Validate(t *testing.T) {
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
			name: "v1alpha1 with cluster CA set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("foo"),
						Key: []byte("bar"),
					},
				},
			},

			expectedError: "kube-apiserver CA is already set in v1alpha1 config (.cluster.ca)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeAPIServerCAConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
