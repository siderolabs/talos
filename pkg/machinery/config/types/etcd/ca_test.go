// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	_ "embed"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/etcd"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

const (
	testCert      = "-----BEGIN CERTIFICATE-----\nMIIC0DCCAbigAwIBAgIUI7z\n-----END CERTIFICATE-----"
	testKey       = "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIABC\n-----END EC PRIVATE KEY-----"
	testAccepted1 = "-----BEGIN CERTIFICATE-----\nMIIACCEPTED1\n-----END CERTIFICATE-----"
	testAccepted2 = "-----BEGIN CERTIFICATE-----\nMIIACCEPTED2\n-----END CERTIFICATE-----"
)

//go:embed testdata/etcdcaconfig.yaml
var expectedEtcdCAConfigDocument []byte

func TestEtcdCAConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := etcd.NewEtcdCAConfigV1Alpha1()
	cfg.CAIssuingCA = &meta.CertificateAndKey{
		Cert: testCert,
		Key:  testKey,
	}
	cfg.CAAcceptedCAs = []string{testAccepted1, testAccepted2}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedEtcdCAConfigDocument, marshaled)
}

func TestEtcdCAConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedEtcdCAConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &etcd.EtcdCAConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       etcd.EtcdCAConfig,
		},
		CAIssuingCA: &meta.CertificateAndKey{
			Cert: testCert,
			Key:  testKey,
		},
		CAAcceptedCAs: []string{testAccepted1, testAccepted2},
	}, docs[0])
}

func TestEtcdCAConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *etcd.EtcdCAConfigV1Alpha1

		expectedError string
	}{
		{
			name: "empty",
			cfg:  etcd.NewEtcdCAConfigV1Alpha1,

			expectedError: "issuing CA is required",
		},
		{
			name: "valid",
			cfg: func() *etcd.EtcdCAConfigV1Alpha1 {
				ca, err := x509.NewSelfSignedCertificateAuthority()
				require.NoError(t, err)

				cfg := etcd.NewEtcdCAConfigV1Alpha1()
				cfg.CAIssuingCA = &meta.CertificateAndKey{
					Cert: string(ca.CrtPEM),
					Key:  string(ca.KeyPEM),
				}
				cfg.CAAcceptedCAs = []string{string(ca.CrtPEM)}

				return cfg
			},
		},
		{
			name: "missing key",
			cfg: func() *etcd.EtcdCAConfigV1Alpha1 {
				ca, err := x509.NewSelfSignedCertificateAuthority()
				require.NoError(t, err)

				cfg := etcd.NewEtcdCAConfigV1Alpha1()
				cfg.CAIssuingCA = &meta.CertificateAndKey{
					Cert: string(ca.CrtPEM),
				}

				return cfg
			},

			expectedError: "issuing CA: key is required",
		},
		{
			name: "invalid issuing CA",
			cfg: func() *etcd.EtcdCAConfigV1Alpha1 {
				cfg := etcd.NewEtcdCAConfigV1Alpha1()
				cfg.CAIssuingCA = &meta.CertificateAndKey{
					Cert: "not a PEM",
					Key:  "not a PEM",
				}

				return cfg
			},

			expectedError: "issuing CA: certificate: no PEM blocks found",
		},
		{
			name: "invalid accepted CA",
			cfg: func() *etcd.EtcdCAConfigV1Alpha1 {
				ca, err := x509.NewSelfSignedCertificateAuthority()
				require.NoError(t, err)

				cfg := etcd.NewEtcdCAConfigV1Alpha1()
				cfg.CAIssuingCA = &meta.CertificateAndKey{
					Cert: string(ca.CrtPEM),
					Key:  string(ca.KeyPEM),
				}
				cfg.CAAcceptedCAs = []string{string(ca.CrtPEM), "not a PEM", string(ca.KeyPEM)}

				return cfg
			},

			expectedError: "accepted CA #1: no PEM blocks found\naccepted CA #2: unexpected PEM block type \"ED25519 PRIVATE KEY\", expected one of [\"CERTIFICATE\"]",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Empty(t, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEtcdCAConfigIssuingCA(t *testing.T) {
	t.Parallel()

	cfg := etcd.NewEtcdCAConfigV1Alpha1()
	assert.Nil(t, cfg.IssuingCA())

	cfg.CAIssuingCA = &meta.CertificateAndKey{
		Cert: "ISSUING",
		Key:  "ISSUING-KEY",
	}

	issuing := cfg.IssuingCA()
	require.NotNil(t, issuing)
	assert.Equal(t, "ISSUING", string(issuing.Crt))
	assert.Equal(t, "ISSUING-KEY", string(issuing.Key))
}

func TestEtcdCAConfigAcceptedCAs(t *testing.T) {
	t.Parallel()

	cfg := etcd.NewEtcdCAConfigV1Alpha1()
	assert.Empty(t, cfg.AcceptedCAs())

	cfg.CAAcceptedCAs = []string{"ACCEPTED1", "ACCEPTED2"}

	accepted := cfg.AcceptedCAs()
	require.Len(t, accepted, 2)
	assert.Equal(t, "ACCEPTED1", string(accepted[0].Crt))
	assert.Equal(t, "ACCEPTED2", string(accepted[1].Crt))

	cfg.CAIssuingCA = &meta.CertificateAndKey{
		Cert: "ISSUING",
		Key:  "ISSUING-KEY",
	}

	// the issuing CA certificate is prepended to the list of accepted CAs
	accepted = cfg.AcceptedCAs()
	require.Len(t, accepted, 3)
	assert.Equal(t, "ISSUING", string(accepted[0].Crt))
	assert.Equal(t, "ACCEPTED1", string(accepted[1].Crt))
	assert.Equal(t, "ACCEPTED2", string(accepted[2].Crt))
}

//go:embed testdata/etcdcaconfig_redacted.yaml
var expectedEtcdCAConfigRedactedDocument []byte

func TestEtcdCAConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := etcd.NewEtcdCAConfigV1Alpha1()
	cfg.CAIssuingCA = &meta.CertificateAndKey{
		Cert: testCert,
		Key:  testKey,
	}
	cfg.CAAcceptedCAs = []string{testAccepted1}

	cfg.Redact("REDACTED")

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedEtcdCAConfigRedactedDocument, marshaled)
}

func TestEtcdCAConfigV1Alpha1ConflictValidate(t *testing.T) {
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
			name: "no etcd",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		},
		{
			name: "v1alpha1 with etcd CA",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					EtcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing deprecated field
						RootCA: &x509.PEMEncodedCertificateAndKey{
							Crt: []byte("foo"),
							Key: []byte("bar"),
						},
					},
				},
			},

			expectedError: "etcd config is already set in v1alpha1 config (.cluster.etcd)",
		},
		{
			name: "v1alpha1 with etcd image",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					EtcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing deprecated field
						ContainerImage: "foo:v1",
					},
				},
			},

			expectedError: "etcd config is already set in v1alpha1 config (.cluster.etcd)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, err := range []error{
				etcd.NewEtcdCAConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg),
				etcd.NewEtcdMemberConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg),
			} {
				if test.expectedError != "" {
					assert.EqualError(t, err, test.expectedError)
				} else {
					assert.NoError(t, err)
				}
			}
		})
	}
}
