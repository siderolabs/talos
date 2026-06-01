// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cosign_test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/pkg/containers/image"
	ourcosign "github.com/siderolabs/talos/internal/pkg/containers/image/verify/internal/cosign"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

type mockRegistriesConfig struct{}

func (mockRegistriesConfig) Mirrors() map[string]config.RegistryMirrorConfig {
	return nil
}

func (mockRegistriesConfig) Auths() map[string]config.RegistryAuthConfig {
	return nil
}

func (mockRegistriesConfig) TLSs() map[string]cri.RegistryTLSConfigExtended {
	return nil
}

func TestVerifyImage(t *testing.T) {
	t.Parallel()

	resolver := image.NewResolver(mockRegistriesConfig{})
	tagFetcher := image.NewTagFetcher(mockRegistriesConfig{})
	trustedRoot, err := cosign.TrustedRoot()
	require.NoError(t, err)

	for _, test := range []struct {
		imageRef string

		checkOpts cosign.CheckOpts

		expectedResultMessage string
		expectedError         string
	}{
		{
			imageRef: "registry.k8s.io/etcd:v3.6.8@sha256:397189418d1a00e500c0605ad18d1baf3b541a1004d768448c367e48071622e5",
			checkOpts: cosign.CheckOpts{
				TrustedMaterial: trustedRoot,
				Identities: []cosign.Identity{
					{
						Issuer:  "https://accounts.google.com",
						Subject: "krel-trust@k8s-releng-prod.iam.gserviceaccount.com",
					},
				},
			},

			expectedResultMessage: "verified via legacy signature (bundle verified true)",
		},
		{
			// Regression for siderolabs/talos#13342: registry.k8s.io's CDN serves the .sig
			// manifest by tag (e.g. us-central1) but can return 404 for the same manifest
			// fetched by digest from another region (e.g. europe-west4). The TagFetcher
			// fallback fetches by tag through the same RegistryHosts (auth/TLS/mirror)
			// the resolver uses.
			imageRef: "registry.k8s.io/etcd:v3.6.11@sha256:fbab3d2954652f592b2653cc1b9decdbe2a633de9320735e9f364b185b6b309a",
			checkOpts: cosign.CheckOpts{
				TrustedMaterial: trustedRoot,
				Identities: []cosign.Identity{
					{
						Issuer:  "https://accounts.google.com",
						Subject: "krel-trust@k8s-releng-prod.iam.gserviceaccount.com",
					},
				},
			},

			expectedResultMessage: "verified via legacy signature (bundle verified true)",
		},
		{
			imageRef: "ghcr.io/siderolabs/talos:v1.13.0-alpha.2@sha256:9361de6684b441da62298998ab89166efccca35772afc24fee3ae53c569ec44c",
			checkOpts: cosign.CheckOpts{
				TrustedMaterial: trustedRoot,
				Identities: []cosign.Identity{
					{
						Issuer:        "https://accounts.google.com",
						SubjectRegExp: "@siderolabs.com$",
					},
				},
			},

			expectedResultMessage: "verified via bundle",
		},
		{
			imageRef: "ghcr.io/siderolabs/extensions:v1.13.0-alpha.2@sha256:033cbce24e681208245797e53386b4bef1c4a995a32feebdfa05c5063f889334",
			checkOpts: cosign.CheckOpts{
				TrustedMaterial: trustedRoot,
				Identities: []cosign.Identity{
					{
						Issuer:  "https://accounts.google.com",
						Subject: "releasemgr-svc@talos-production.iam.gserviceaccount.com",
					},
				},
			},

			expectedResultMessage: "verified via bundle",
		},
		{
			imageRef: "ghcr.io/siderolabs/extensions:v1.13.0-alpha.1-17-gc538dab@sha256:32ed7bb3845215bfd71bf4284a2a5113ecd49ce45cde0324764fe84b378c8633",
			checkOpts: cosign.CheckOpts{
				TrustedMaterial: trustedRoot,
				Identities: []cosign.Identity{
					{
						Issuer:  "https://accounts.google.com",
						Subject: "releasemgr-svc@talos-production.iam.gserviceaccount.com",
					},
				},
			},

			expectedError: "no valid signature found: bundle tag not found\nlegacy signature tag not found",
		},
		{
			imageRef: "ghcr.io/siderolabs/extensions:v1.13.0-alpha.1@sha256:5c3abcee03ef7369bb92f1f3d76c1afd27ccc97fa2879145a486b554f3091648",
			checkOpts: cosign.CheckOpts{
				TrustedMaterial: trustedRoot,
				Identities: []cosign.Identity{
					{
						Issuer:  "https://some.entity",
						Subject: "releasemgr@world",
					},
				},
			},

			expectedError: "no valid bundle layer: failed to verify certificate identity: no matching CertificateIdentity found, last error: expected SAN " +
				"value \"releasemgr@world\", got \"releasemgr-svc@talos-production.iam.gserviceaccount.com\"",
		},
	} {
		t.Run(test.imageRef, func(t *testing.T) {
			t.Parallel()

			logger := zaptest.NewLogger(t)

			imageRef, err := name.NewDigest(test.imageRef)
			require.NoError(t, err)

			result, err := ourcosign.VerifyImage(t.Context(), logger, resolver, tagFetcher, imageRef, test.checkOpts)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedError)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, test.expectedResultMessage, result.Message)
		})
	}
}
