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

//go:embed testdata/authenticationconfig.yaml
var expectedKubeAuthenticationConfigDocument []byte

func TestKubeAuthenticationConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := k8s.NewKubeAuthenticationConfigV1Alpha1()
	cfg.AuthConfig.Object = map[string]any{
		"apiVersion": "apiserver.config.k8s.io/v1beta1",
		"kind":       "AuthenticationConfiguration",
		"jwt": []any{
			map[string]any{
				"issuer": map[string]any{
					"url":       "https://example.com",
					"audiences": []any{"my-app"},
				},
			},
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeAuthenticationConfigDocument, marshaled)
}

func TestKubeAuthenticationConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeAuthenticationConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &k8s.KubeAuthenticationConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       k8s.KubeAuthenticationConfig,
		},
		AuthConfig: meta.Unstructured{
			Object: map[string]any{
				"apiVersion": "apiserver.config.k8s.io/v1beta1",
				"kind":       "AuthenticationConfiguration",
				"jwt": []any{
					map[string]any{
						"issuer": map[string]any{
							"url":       "https://example.com",
							"audiences": []any{"my-app"},
						},
					},
				},
			},
		},
	}, docs[0])
}
