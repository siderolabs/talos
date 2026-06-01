// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

//go:embed testdata/registryauthconfig.yaml
var expectedRegistryAuthConfigDocument []byte

func TestRegistryAuthConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := cri.NewRegistryAuthConfigV1Alpha1("my-private-registry.io")
	cfg.RegistryUsername = "agent007"
	cfg.RegistryPassword = "topsecret"

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedRegistryAuthConfigDocument, marshaled)
}

func TestRegistryAuthConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedRegistryAuthConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &cri.RegistryAuthConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       cri.RegistryAuthConfig,
		},
		MetaName:         "my-private-registry.io",
		RegistryUsername: "agent007",
		RegistryPassword: "topsecret",
	}, docs[0])
}

func TestRegistryAuthConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *cri.RegistryAuthConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *cri.RegistryAuthConfigV1Alpha1 {
				return cri.NewRegistryAuthConfigV1Alpha1("")
			},

			expectedError: "name must be specified",
		},
		{
			name: "username and identity token",
			cfg: func() *cri.RegistryAuthConfigV1Alpha1 {
				cfg := cri.NewRegistryAuthConfigV1Alpha1("k8s.io")
				cfg.RegistryUsername = "user"
				cfg.RegistryIdentityToken = "token"

				return cfg
			},

			expectedError: "only one of username/password or identityToken authentication can be specified",
		},
		{
			name: "valid",
			cfg: func() *cri.RegistryAuthConfigV1Alpha1 {
				cfg := cri.NewRegistryAuthConfigV1Alpha1("k8s.io")
				cfg.RegistryUsername = "user"
				cfg.RegistryPassword = "pass"

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
