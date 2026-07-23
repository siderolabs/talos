// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

//go:embed testdata/inlinemanifestconfig.yaml
var expectedKubeInlineManifestConfigDocument []byte

// multiDocManifest is a multi-line Kubernetes manifest holding several YAML documents
// separated by `---`, matching what can be supplied to `kubectl apply -f <file>`.
const multiDocManifest = `apiVersion: v1
kind: Namespace
metadata:
  name: ci
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: build-settings
  namespace: ci
data:
  parallelism: "4"
  verbose: "true"`

func TestKubeInlineManifestConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeInlineManifestConfigV1Alpha1()
	cfg.MetaName = "namespace-ci"
	cfg.ManifestSpec = multiDocManifest

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeInlineManifestConfigDocument, marshaled)
}

func TestKubeInlineManifestConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeInlineManifestConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeInlineManifestConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeInlineManifestConfig,
		},
		MetaName:     "namespace-ci",
		ManifestSpec: multiDocManifest,
	}, docs[0])
}

func TestKubeInlineManifestConfigMultiLineRoundtrip(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeInlineManifestConfigV1Alpha1()
	cfg.MetaName = "namespace-ci"
	cfg.ManifestSpec = multiDocManifest

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	provider, err := configloader.NewFromBytes(marshaled)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	decoded, ok := docs[0].(*k8s.KubeInlineManifestConfigV1Alpha1)
	require.True(t, ok)

	// the multi-line manifest, including the `---` document separator, must survive
	// the marshal/unmarshal roundtrip unchanged.
	assert.Equal(t, multiDocManifest, decoded.Contents())
	assert.Equal(t, "namespace-ci", decoded.Name())
}

func TestKubeInlineManifestConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeInlineManifestConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeInlineManifestConfigV1Alpha1,

			expectedError: "manifest name is required",
		},
		{
			name: "invalid name",
			cfg: func() *k8s.KubeInlineManifestConfigV1Alpha1 {
				cfg := k8s.NewKubeInlineManifestConfigV1Alpha1()
				cfg.MetaName = "Invalid_Name"

				return cfg
			},

			expectedError: "manifest name is invalid: domain doesn't match required format: \"Invalid_Name\"",
		},
		{
			name: "valid",
			cfg: func() *k8s.KubeInlineManifestConfigV1Alpha1 {
				cfg := k8s.NewKubeInlineManifestConfigV1Alpha1()
				cfg.MetaName = "namespace-ci"
				cfg.ManifestSpec = multiDocManifest

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
