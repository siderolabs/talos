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

//go:embed testdata/staticpodconfig.yaml
var expectedKubeStaticPodConfigDocument []byte

func staticPodSpec() map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name": "nginx",
		},
		"spec": map[string]any{
			"containers": []any{
				map[string]any{
					"name":  "nginx",
					"image": "nginx",
				},
			},
		},
	}
}

func TestKubeStaticPodConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeStaticPodConfigV1Alpha1()
	cfg.MetaName = "nginx"
	cfg.PodSpec.Object = staticPodSpec()

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeStaticPodConfigDocument, marshaled)
}

func TestKubeStaticPodConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeStaticPodConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeStaticPodConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeStaticPodConfig,
		},
		MetaName: "nginx",
		PodSpec: meta.Unstructured{
			Object: staticPodSpec(),
		},
	}, docs[0])
}

func TestKubeStaticPodConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeStaticPodConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  k8s.NewKubeStaticPodConfigV1Alpha1,

			expectedError: "static pod name is required\nstatic pod spec is required",
		},
		{
			name: "invalid name",
			cfg: func() *k8s.KubeStaticPodConfigV1Alpha1 {
				cfg := k8s.NewKubeStaticPodConfigV1Alpha1()
				cfg.MetaName = "Invalid_Name"
				cfg.PodSpec.Object = staticPodSpec()

				return cfg
			},

			expectedError: "static pod name is invalid: domain doesn't match required format: \"Invalid_Name\"",
		},
		{
			name: "valid",
			cfg: func() *k8s.KubeStaticPodConfigV1Alpha1 {
				cfg := k8s.NewKubeStaticPodConfigV1Alpha1()
				cfg.MetaName = "nginx"
				cfg.PodSpec.Object = staticPodSpec()

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
