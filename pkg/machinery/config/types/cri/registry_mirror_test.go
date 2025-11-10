// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	_ "embed"
	"net/url"
	"testing"

	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

//go:embed testdata/registrymirrorconfig.yaml
var expectedRegistryMirrorConfigDocument []byte

func TestRegistryMirrorConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := cri.NewRegistryMirrorConfigV1Alpha1("ghcr.io")
	cfg.RegistrySkipFallback = pointer.To(true)
	cfg.RegistryEndpoints = []cri.RegistryEndpoint{
		{
			EndpointURL: meta.URL{URL: ensure.Value(url.Parse("https://my-private-registry.local:5000"))},
		},
		{
			EndpointURL:          meta.URL{URL: ensure.Value(url.Parse("http://my-harbor/v2/registry-k8s.io/"))},
			EndpointOverridePath: pointer.To(true),
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedRegistryMirrorConfigDocument, marshaled)
}

func TestRegistryMirrorConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedRegistryMirrorConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &cri.RegistryMirrorConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       cri.RegistryMirrorConfig,
		},
		MetaName:             "ghcr.io",
		RegistrySkipFallback: pointer.To(true),
		RegistryEndpoints: []cri.RegistryEndpoint{
			{
				EndpointURL: meta.URL{URL: ensure.Value(url.Parse("https://my-private-registry.local:5000"))},
			},
			{
				EndpointURL:          meta.URL{URL: ensure.Value(url.Parse("http://my-harbor/v2/registry-k8s.io/"))},
				EndpointOverridePath: pointer.To(true),
			},
		},
	}, docs[0])
}

func TestRegistryMirrorConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *cri.RegistryMirrorConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg: func() *cri.RegistryMirrorConfigV1Alpha1 {
				return cri.NewRegistryMirrorConfigV1Alpha1("")
			},

			expectedError: "name must be specified",
		},
		{
			name: "invalid endpoint URL",
			cfg: func() *cri.RegistryMirrorConfigV1Alpha1 {
				cfg := cri.NewRegistryMirrorConfigV1Alpha1("docker.io")
				cfg.RegistryEndpoints = []cri.RegistryEndpoint{
					{
						EndpointURL: meta.URL{},
					},
				}

				return cfg
			},

			expectedError: "endpoints[0].url must be specified",
		},
		{
			name: "unsupported endpoint URL scheme",
			cfg: func() *cri.RegistryMirrorConfigV1Alpha1 {
				cfg := cri.NewRegistryMirrorConfigV1Alpha1("docker.io")
				cfg.RegistryEndpoints = []cri.RegistryEndpoint{
					{
						EndpointURL: meta.URL{URL: ensure.Value(url.Parse("ftp://my-registry.local:5000"))},
					},
				}

				return cfg
			},

			expectedError: "endpoints[0].url has unsupported scheme: \"ftp\"",
		},
		{
			name: "valid empty endpoints",
			cfg: func() *cri.RegistryMirrorConfigV1Alpha1 {
				cfg := cri.NewRegistryMirrorConfigV1Alpha1("docker.io")

				return cfg
			},
		},
		{
			name: "valid",
			cfg: func() *cri.RegistryMirrorConfigV1Alpha1 {
				cfg := cri.NewRegistryMirrorConfigV1Alpha1("gcr.io")
				cfg.RegistryEndpoints = []cri.RegistryEndpoint{
					{
						EndpointURL: meta.URL{URL: ensure.Value(url.Parse("https://my-private-registry.local:5000"))},
					},
					{
						EndpointURL:          meta.URL{URL: ensure.Value(url.Parse("http://my-harbor/v2/registry-k8s.io/"))},
						EndpointOverridePath: pointer.To(true),
					},
				}

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

type validationMode struct{}

func (validationMode) String() string {
	return ""
}

func (validationMode) RequiresInstall() bool {
	return false
}

func (validationMode) InContainer() bool {
	return false
}
