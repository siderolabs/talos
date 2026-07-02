// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	_ "embed"
	"net/url"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/serviceaccountconfig.yaml
var expectedKubeServiceAccountConfigDocument []byte

func serviceAccountConfig() *k8s.KubeServiceAccountConfigV1Alpha1 {
	cfg := k8s.NewKubeServiceAccountConfigV1Alpha1()
	cfg.ServiceIssuer = k8s.IssuerServiceAccountConfig{
		PrivateKey: "PRIVATE-KEY",
		IssuerURL:  meta.URL{URL: ensure.Value(url.Parse("https://my-control-plane:6443"))},
	}
	cfg.ServiceAccepted = k8s.AcceptedServiceAccountConfig{
		PublicKeys: []string{"PUBLIC-KEY-1", "PUBLIC-KEY-2"},
		Issuers:    []meta.URL{{URL: ensure.Value(url.Parse("https://another-control-plane:6443"))}},
		Audiences:  []string{"https://another-control-plane:6443", "https://my-control-plane:6443"},
	}

	return cfg
}

func TestKubeServiceAccountConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := serviceAccountConfig()

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeServiceAccountConfigDocument, marshaled)
}

func TestKubeServiceAccountConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeServiceAccountConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, serviceAccountConfig(), docs[0])
}

func TestKubeServiceAccountConfigValidate(t *testing.T) {
	t.Parallel()

	ca, err := x509.NewSelfSignedCertificateAuthority()
	require.NoError(t, err)

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeServiceAccountConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeServiceAccountConfigV1Alpha1,

			expectedError: "service issuer private key is required\nservice issuer URL is required",
		},
		{
			name: "valid",
			cfg: func() *k8s.KubeServiceAccountConfigV1Alpha1 {
				cfg := k8s.NewKubeServiceAccountConfigV1Alpha1()
				cfg.ServiceIssuer = k8s.IssuerServiceAccountConfig{
					PrivateKey: string(ca.KeyPEM),
					IssuerURL:  meta.URL{URL: ensure.Value(url.Parse("https://my-control-plane:6443"))},
				}

				return cfg
			},
		},
		{
			name: "missing issuer URL",
			cfg: func() *k8s.KubeServiceAccountConfigV1Alpha1 {
				cfg := k8s.NewKubeServiceAccountConfigV1Alpha1()
				cfg.ServiceIssuer.PrivateKey = string(ca.KeyPEM)

				return cfg
			},

			expectedError: "service issuer URL is required",
		},
		{
			name: "invalid private key",
			cfg: func() *k8s.KubeServiceAccountConfigV1Alpha1 {
				cfg := k8s.NewKubeServiceAccountConfigV1Alpha1()
				cfg.ServiceIssuer = k8s.IssuerServiceAccountConfig{
					PrivateKey: "not a PEM",
					IssuerURL:  meta.URL{URL: ensure.Value(url.Parse("https://my-control-plane:6443"))},
				}

				return cfg
			},

			expectedError: "service issuer private key is invalid: failed to parse PEM block",
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

//go:embed testdata/serviceaccountconfig_redacted.yaml
var expectedKubeServiceAccountConfigRedactedDocument []byte

func TestKubeServiceAccountConfigRedact(t *testing.T) {
	t.Parallel()

	cfg := serviceAccountConfig()

	cfg.Redact("REDACTED")

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeServiceAccountConfigRedactedDocument, marshaled)
}

func TestKubeServiceAccountConfigRedactEmpty(t *testing.T) {
	t.Parallel()

	// redacting an empty private key should be a no-op
	cfg := k8s.NewKubeServiceAccountConfigV1Alpha1()
	cfg.Redact("REDACTED")

	assert.Empty(t, cfg.ServiceIssuer.PrivateKey)
}

func TestKubeServiceAccountConfigIssuingKey(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeServiceAccountConfigV1Alpha1()
	cfg.ServiceIssuer.PrivateKey = "PRIVATE-KEY"

	assert.Equal(t, "PRIVATE-KEY", string(cfg.IssuingKey().Key))
}

func TestKubeServiceAccountConfigAcceptedKeys(t *testing.T) {
	t.Parallel()

	ca, err := x509.NewSelfSignedCertificateAuthority()
	require.NoError(t, err)

	cfg := k8s.NewKubeServiceAccountConfigV1Alpha1()
	cfg.ServiceIssuer.PrivateKey = string(ca.KeyPEM)
	cfg.ServiceAccepted.PublicKeys = []string{"PUBLIC-KEY-1", "PUBLIC-KEY-2"}

	accepted := cfg.AcceptedKeys()
	require.Len(t, accepted, 3)

	// the public key derived from the issuing private key is prepended to the list of accepted keys
	issuingKey, err := cfg.IssuingKey().GetKey()
	require.NoError(t, err)

	assert.Equal(t, string(issuingKey.GetPublicKeyPEM()), string(accepted[0].Key))
	assert.Equal(t, "PUBLIC-KEY-1", string(accepted[1].Key))
	assert.Equal(t, "PUBLIC-KEY-2", string(accepted[2].Key))
}

func TestKubeServiceAccountConfigIssuerURL(t *testing.T) {
	t.Parallel()

	cfg := serviceAccountConfig()

	assert.Equal(t, "https://my-control-plane:6443", cfg.IssuerURL())
}

func TestKubeServiceAccountConfigAcceptedIssuers(t *testing.T) {
	t.Parallel()

	cfg := serviceAccountConfig()

	assert.Equal(t, []string{
		"https://another-control-plane:6443",
	}, cfg.AcceptedIssuers())
}

func TestKubeServiceAccountConfigAPIAudiences(t *testing.T) {
	t.Parallel()

	cfg := serviceAccountConfig()

	assert.Equal(t, []string{
		"https://another-control-plane:6443",
		"https://my-control-plane:6443",
	}, cfg.APIAudiences())

	// when no audiences are set, the issuer URL is used as the default
	cfg.ServiceAccepted.Audiences = nil

	assert.Equal(t, []string{"https://my-control-plane:6443"}, cfg.APIAudiences())
}

//nolint:dupl
func TestKubeServiceAccountConfigV1Alpha1Validate(t *testing.T) {
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
			name: "v1alpha1 with cluster service account set",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterServiceAccount: &x509.PEMEncodedKey{
						Key: []byte("foo"),
					},
				},
			},

			expectedError: "service account is already set in v1alpha1 config (.cluster.serviceAccount)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := k8s.NewKubeServiceAccountConfigV1Alpha1().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
