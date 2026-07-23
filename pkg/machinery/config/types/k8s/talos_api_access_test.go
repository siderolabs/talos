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
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

//go:embed testdata/kubetalosapiaccessconfig.yaml
var expectedKubeTalosAPIAccessConfigDocument []byte

func kubeTalosAPIAccessConfig() *k8s.KubeTalosAPIAccessConfigV1Alpha1 {
	cfg := k8s.NewKubeTalosAPIAccessConfigV1Alpha1()
	cfg.AccessAllowedRoles = []string{string(role.Reader)}
	cfg.AccessAllowedKubernetesNamespaces = []string{"kube-system"}

	return cfg
}

func TestKubeTalosAPIAccessConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := kubeTalosAPIAccessConfig()

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeTalosAPIAccessConfigDocument, marshaled)
}

func TestKubeTalosAPIAccessConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeTalosAPIAccessConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, kubeTalosAPIAccessConfig(), docs[0])
}

func TestKubeTalosAPIAccessConfigAccessors(t *testing.T) {
	t.Parallel()

	cfg := kubeTalosAPIAccessConfig()

	assert.Equal(t, []string{"os:reader"}, cfg.AllowedRoles())
	assert.Equal(t, []string{"kube-system"}, cfg.AllowedKubernetesNamespaces())

	empty := k8s.NewKubeTalosAPIAccessConfigV1Alpha1()

	assert.Empty(t, empty.AllowedRoles())
	assert.Empty(t, empty.AllowedKubernetesNamespaces())
}

func TestKubeTalosAPIAccessConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeTalosAPIAccessConfigV1Alpha1

		expectedError string
	}{
		{
			name: "valid",
			cfg:  kubeTalosAPIAccessConfig,
		},
		{
			name: "no roles",
			cfg:  k8s.NewKubeTalosAPIAccessConfigV1Alpha1,
		},
		{
			name: "all roles",
			cfg: func() *k8s.KubeTalosAPIAccessConfigV1Alpha1 {
				cfg := k8s.NewKubeTalosAPIAccessConfigV1Alpha1()
				cfg.AccessAllowedRoles = []string{
					string(role.Admin),
					string(role.Operator),
					string(role.Reader),
					string(role.EtcdBackup),
					string(role.ImageVerifier),
					string(role.MetaWriter),
					string(role.Impersonator),
				}

				return cfg
			},
		},
		{
			name: "invalid role",
			cfg: func() *k8s.KubeTalosAPIAccessConfigV1Alpha1 {
				cfg := k8s.NewKubeTalosAPIAccessConfigV1Alpha1()
				cfg.AccessAllowedRoles = []string{"os:superuser"}

				return cfg
			},
			expectedError: "invalid role \"os:superuser\" in .allowedRoles",
		},
		{
			name: "multiple invalid roles",
			cfg: func() *k8s.KubeTalosAPIAccessConfigV1Alpha1 {
				cfg := k8s.NewKubeTalosAPIAccessConfigV1Alpha1()
				cfg.AccessAllowedRoles = []string{"reader", string(role.Reader), ""}

				return cfg
			},
			expectedError: "invalid role \"reader\" in .allowedRoles\ninvalid role \"\" in .allowedRoles",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})
			assert.Nil(t, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKubeTalosAPIAccessConfigV1Alpha1ConflictValidate(t *testing.T) {
	t.Parallel()

	cfg := kubeTalosAPIAccessConfig()

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
			name: "empty machine features",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{}, //nolint:staticcheck // testing legacy config conflict
				},
			},
		},
		{
			name: "legacy Talos API access present",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{ //nolint:staticcheck // testing legacy config conflict
						KubernetesTalosAPIAccessConfig: &v1alpha1.KubernetesTalosAPIAccessConfig{}, //nolint:staticcheck // testing legacy config conflict
					},
				},
			},
			expectedError: ".machine.features.kubernetesTalosAPIAccess is already set in v1alpha1 config",
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
