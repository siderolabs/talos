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

	ourcosign "github.com/siderolabs/talos/internal/app/images/internal/cosign"
	"github.com/siderolabs/talos/internal/pkg/containers/image"
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
	trustedRoot, err := cosign.TrustedRoot()
	require.NoError(t, err)

	for _, test := range []struct {
		imageRef string

		checkOpts cosign.CheckOpts

		expectedResultMessage string
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

			expectedResultMessage: "verified via legacy signature (bundle verified true) layer with digest sha256:5e2fb670e3a47d4556a85434d8d5db220fd203a5fb2c99817f5ffc1ada8cf030",
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

			expectedResultMessage: "verified via bundle layer with digest sha256:7754e217f744fde6bf840f91a6cf74c8d5d8920884674ed3ecc3c25499833a92",
		},
		{
			imageRef: "ghcr.io/siderolabs/extensions:v1.13.0-alpha.2@sha256:033cbce24e681208245797e53386b4bef1c4a995a32feebdfa05c5063f889334",
			checkOpts: cosign.CheckOpts{
				TrustedMaterial: trustedRoot,
				Identities: []cosign.Identity{ // will not verify
					{
						Issuer:  "https://my-service",
						Subject: "releasemgr@universe",
					},
					{
						Issuer:  "https://accounts.google.com",
						Subject: "releasemgr-svc@talos-production.iam.gserviceaccount.com",
					},
				},
			},

			expectedResultMessage: "verified via bundle layer with digest sha256:17ce03f46eff246dac6ea95e056a2e80da7e324e7928121fa8999c0cb8ea30bf",
		},
	} {
		t.Run(test.imageRef, func(t *testing.T) {
			t.Parallel()

			logger := zaptest.NewLogger(t)

			imageRef, err := name.NewDigest(test.imageRef)
			require.NoError(t, err)

			result, err := ourcosign.VerifyImage(t.Context(), logger, resolver, imageRef, test.checkOpts)
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, test.expectedResultMessage, result.Message)

			t.Logf("verification result: %s", result.Message)
		})
	}
}
