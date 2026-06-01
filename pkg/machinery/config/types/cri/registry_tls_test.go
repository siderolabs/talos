// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	_ "embed"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

//go:embed testdata/registrytlsconfig.yaml
var expectedRegistryTLSConfigDocument []byte

func TestRegistryTLSConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := cri.NewRegistryTLSConfigV1Alpha1("my-tls-registry.io")
	cfg.TLSCA = "-----BEGIN CERTIFICATE-----\nMIID...IDAQAB\n-----END CERTIFICATE-----"
	cfg.TLSClientIdentity = &meta.CertificateAndKey{
		Cert: "-----BEGIN CERTIFICATE-----\nMIID...IDAQAB\n-----END CERTIFICATE-----",
		Key:  "-----BEGIN PRIVATE KEY-----\nMIIE...AB\n-----END PRIVATE KEY-----",
	}
	cfg.TLSInsecureSkipVerify = new(true)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedRegistryTLSConfigDocument, marshaled)
}

func TestRegistryTLSConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedRegistryTLSConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &cri.RegistryTLSConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       cri.RegistryTLSConfig,
		},
		MetaName: "my-tls-registry.io",
		TLSCA:    "-----BEGIN CERTIFICATE-----\nMIID...IDAQAB\n-----END CERTIFICATE-----",
		TLSClientIdentity: &meta.CertificateAndKey{
			Cert: "-----BEGIN CERTIFICATE-----\nMIID...IDAQAB\n-----END CERTIFICATE-----",
			Key:  "-----BEGIN PRIVATE KEY-----\nMIIE...AB\n-----END PRIVATE KEY-----",
		},
		TLSInsecureSkipVerify: new(true),
	}, docs[0])
}

func TestRegistryTLSConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *cri.RegistryTLSConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *cri.RegistryTLSConfigV1Alpha1 {
				return cri.NewRegistryTLSConfigV1Alpha1("")
			},

			expectedError: "name must be specified",
		},
		{
			name: "valid small",
			cfg: func() *cri.RegistryTLSConfigV1Alpha1 {
				cfg := cri.NewRegistryTLSConfigV1Alpha1("rr.k8s.io")
				cfg.TLSInsecureSkipVerify = new(true)

				return cfg
			},
		},
		{
			name: "valid",
			cfg: func() *cri.RegistryTLSConfigV1Alpha1 {
				cfg := cri.NewRegistryTLSConfigV1Alpha1("k8s.io")

				ca, err := x509.NewSelfSignedCertificateAuthority()
				require.NoError(t, err)

				cfg.TLSCA = string(ca.CrtPEM)
				cfg.TLSClientIdentity = &meta.CertificateAndKey{
					Cert: string(ca.CrtPEM),
					Key:  string(ca.KeyPEM),
				}
				cfg.TLSInsecureSkipVerify = new(false)

				return cfg
			},
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
