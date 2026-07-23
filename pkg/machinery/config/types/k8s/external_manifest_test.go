// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	_ "embed"
	"net/url"
	"testing"

	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

//go:embed testdata/externalmanifestconfig.yaml
var expectedKubeExternalManifestConfigDocument []byte

func TestKubeExternalManifestConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeExternalManifestConfigV1Alpha1()
	cfg.MetaName = "example-cni"
	cfg.HeadersSpec = map[string]string{
		"Authorization": "Bearer token",
	}
	cfg.URLSpec = meta.URL{URL: ensure.Value(url.Parse("https://www.example.com/manifest1.yaml"))}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeExternalManifestConfigDocument, marshaled)
}

func TestKubeExternalManifestConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeExternalManifestConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeExternalManifestConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeExternalManifestConfig,
		},
		MetaName: "example-cni",
		HeadersSpec: map[string]string{
			"Authorization": "Bearer token",
		},
		URLSpec: meta.URL{URL: ensure.Value(url.Parse("https://www.example.com/manifest1.yaml"))},
	}, docs[0])
}

func TestKubeExternalManifestConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeExternalManifestConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeExternalManifestConfigV1Alpha1,

			expectedError: "manifest name is required\nmanifest URL is required",
		},
		{
			name: "invalid name",
			cfg: func() *k8s.KubeExternalManifestConfigV1Alpha1 {
				cfg := k8s.NewKubeExternalManifestConfigV1Alpha1()
				cfg.MetaName = "Invalid_Name"
				cfg.URLSpec = meta.URL{URL: ensure.Value(url.Parse("https://www.example.com/manifest1.yaml"))}

				return cfg
			},

			expectedError: "manifest name is invalid: domain doesn't match required format: \"Invalid_Name\"",
		},
		{
			name: "missing URL",
			cfg: func() *k8s.KubeExternalManifestConfigV1Alpha1 {
				cfg := k8s.NewKubeExternalManifestConfigV1Alpha1()
				cfg.MetaName = "example-cni"

				return cfg
			},

			expectedError: "manifest URL is required",
		},
		{
			name: "valid",
			cfg: func() *k8s.KubeExternalManifestConfigV1Alpha1 {
				cfg := k8s.NewKubeExternalManifestConfigV1Alpha1()
				cfg.MetaName = "example-cni"
				cfg.URLSpec = meta.URL{URL: ensure.Value(url.Parse("https://www.example.com/manifest1.yaml"))}

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
