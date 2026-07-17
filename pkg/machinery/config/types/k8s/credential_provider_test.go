// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
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
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/kubecredentialproviderconfig.yaml
var expectedKubeCredentialProviderConfigDocument []byte

func kubeCredentialProviderConfig() *k8s.KubeCredentialProviderConfigV1Alpha1 {
	cfg := k8s.NewKubeCredentialProviderConfigV1Alpha1()
	cfg.CredentialProviderConfig = meta.Unstructured{
		Object: map[string]any{
			"apiVersion": "kubelet.config.k8s.io/v1",
			"kind":       "CredentialProviderConfig",
			"providers": []any{
				map[string]any{
					"name":       "ecr-credential-provider",
					"apiVersion": "credentialprovider.kubelet.k8s.io/v1",
					"matchImages": []any{
						"*.dkr.ecr.*.amazonaws.com",
					},
					"defaultCacheDuration": "12h",
				},
			},
		},
	}

	return cfg
}

func TestKubeCredentialProviderConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := kubeCredentialProviderConfig()

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeCredentialProviderConfigDocument, marshaled)
}

func TestKubeCredentialProviderConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeCredentialProviderConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, kubeCredentialProviderConfig(), docs[0])
}

func TestKubeCredentialProviderConfigAccessors(t *testing.T) {
	t.Parallel()

	cfg := kubeCredentialProviderConfig()

	assert.Equal(t, map[string]any{
		"apiVersion": "kubelet.config.k8s.io/v1",
		"kind":       "CredentialProviderConfig",
		"providers": []any{
			map[string]any{
				"name":       "ecr-credential-provider",
				"apiVersion": "credentialprovider.kubelet.k8s.io/v1",
				"matchImages": []any{
					"*.dkr.ecr.*.amazonaws.com",
				},
				"defaultCacheDuration": "12h",
			},
		},
	}, cfg.Configuration())
}

func TestKubeCredentialProviderConfigV1Alpha1ConflictValidate(t *testing.T) {
	t.Parallel()

	cfg := kubeCredentialProviderConfig()

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
			name: "empty machine config",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{},
			},
		},
		{
			name: "legacy kubelet present",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineKubelet: &v1alpha1.KubeletConfig{}, //nolint:staticcheck // testing legacy config conflict
				},
			},
			expectedError: "cannot use KubeCredentialProviderConfig with legacy kubelet configuration (.machine.kubelet)",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := cfg.V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
