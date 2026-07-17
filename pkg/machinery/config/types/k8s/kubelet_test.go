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
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//go:embed testdata/kubeletconfig.yaml
var expectedKubeletConfigDocument []byte

func kubeletConfig() *k8s.KubeletConfigV1Alpha1 {
	cfg := k8s.NewKubeletConfigV1Alpha1()
	cfg.KubeletImage = constants.KubeletImage + ":v1.36.0"
	cfg.KubeletConfig = meta.Unstructured{
		Object: map[string]any{
			"serverTLSBootstrap": true,
		},
	}
	cfg.KubeletArgs = meta.Args{
		"feature-gates": meta.NewArgValue("AllBeta=true", nil),
	}
	cfg.KubeletClusterDNS = []string{"10.96.0.10"}
	cfg.KubeletDefaultRuntimeSeccompProfileEnabled = new(true)

	return cfg
}

func TestKubeletConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := kubeletConfig()

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeletConfigDocument, marshaled)
}

func TestKubeletConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubeletConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, kubeletConfig(), docs[0])
}

func TestKubeletConfigAccessors(t *testing.T) {
	t.Parallel()

	cfg := kubeletConfig()

	assert.Equal(t, constants.KubeletImage+":v1.36.0", cfg.Image())
	assert.Equal(t, []string{"10.96.0.10"}, cfg.ClusterDNS())
	assert.Equal(t, map[string][]string{"feature-gates": {"AllBeta=true"}}, cfg.ExtraArgs())
	assert.Nil(t, cfg.ExtraMounts())
	assert.Equal(t, map[string]any{"serverTLSBootstrap": true}, cfg.ExtraConfig())
	assert.True(t, cfg.DefaultRuntimeSeccompProfileEnabled())
	assert.True(t, cfg.DisableManifestsDirectory())

	empty := k8s.NewKubeletConfigV1Alpha1()

	assert.False(t, empty.DefaultRuntimeSeccompProfileEnabled())
}

func TestKubeletConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubeletConfigV1Alpha1

		onMachineMode bool

		expectedError string
	}{
		{
			name:          "valid",
			cfg:           kubeletConfig,
			onMachineMode: true,
		},
		{
			name: "valid, local",
			cfg: func() *k8s.KubeletConfigV1Alpha1 {
				cfg := k8s.NewKubeletConfigV1Alpha1()
				cfg.KubeletImage = "not-a-valid-tag"

				return cfg
			},
		},
		{
			name:          "empty image",
			cfg:           k8s.NewKubeletConfigV1Alpha1,
			onMachineMode: true,
			expectedError: "kubelet image cannot be empty",
		},
		{
			name: "invalid image tag",
			cfg: func() *k8s.KubeletConfigV1Alpha1 {
				cfg := k8s.NewKubeletConfigV1Alpha1()
				cfg.KubeletImage = constants.KubeletImage

				return cfg
			},
			onMachineMode: true,
			expectedError: "kubelet image is not valid: failed to parse Kubernetes version from image reference \"" + constants.KubeletImage +
				"\": invalid image reference: \"" + constants.KubeletImage + "\"",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var validationOptions []validation.Option

			if !test.onMachineMode {
				validationOptions = append(validationOptions, validation.WithLocal())
			}

			warnings, err := test.cfg().Validate(validationMode{}, validationOptions...)
			assert.Nil(t, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKubeletConfigV1Alpha1ConflictValidate(t *testing.T) {
	t.Parallel()

	cfg := kubeletConfig()

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
			expectedError: "kubelet config is already set in v1alpha1 config (.machine.kubelet)",
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
