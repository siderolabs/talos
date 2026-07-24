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
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//go:embed testdata/kubeprismconfig.yaml
var expectedKubePrismConfigDocument []byte

func kubePrismConfig() *k8s.KubePrismConfigV1Alpha1 {
	cfg := k8s.NewKubePrismConfigV1Alpha1()
	cfg.PortConfig = constants.DefaultKubePrismPort
	cfg.TLSServerNameConfig = "api.cluster.local"

	return cfg
}

func TestKubePrismConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := kubePrismConfig()

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubePrismConfigDocument, marshaled)
}

func TestKubePrismConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedKubePrismConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, kubePrismConfig(), docs[0])
}

func TestKubePrismConfigAccessors(t *testing.T) {
	t.Parallel()

	cfg := kubePrismConfig()

	assert.Equal(t, constants.DefaultKubePrismPort, cfg.Port())
	assert.Equal(t, "api.cluster.local", cfg.TLSServerName())

	empty := k8s.NewKubePrismConfigV1Alpha1()

	assert.Equal(t, 0, empty.Port())
	assert.Empty(t, empty.TLSServerName())
}

func TestKubePrismConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *k8s.KubePrismConfigV1Alpha1

		expectedError string
	}{
		{
			name: "valid",
			cfg:  kubePrismConfig,
		},
		{
			name: "minimal port",
			cfg: func() *k8s.KubePrismConfigV1Alpha1 {
				cfg := k8s.NewKubePrismConfigV1Alpha1()
				cfg.PortConfig = 1

				return cfg
			},
		},
		{
			name: "maximal port",
			cfg: func() *k8s.KubePrismConfigV1Alpha1 {
				cfg := k8s.NewKubePrismConfigV1Alpha1()
				cfg.PortConfig = 65535

				return cfg
			},
		},
		{
			name:          "no port",
			cfg:           k8s.NewKubePrismConfigV1Alpha1,
			expectedError: "invalid port 0: must be in range 1-65535",
		},
		{
			name: "negative port",
			cfg: func() *k8s.KubePrismConfigV1Alpha1 {
				cfg := k8s.NewKubePrismConfigV1Alpha1()
				cfg.PortConfig = -1

				return cfg
			},
			expectedError: "invalid port -1: must be in range 1-65535",
		},
		{
			name: "port out of range",
			cfg: func() *k8s.KubePrismConfigV1Alpha1 {
				cfg := k8s.NewKubePrismConfigV1Alpha1()
				cfg.PortConfig = 65536

				return cfg
			},
			expectedError: "invalid port 65536: must be in range 1-65535",
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

func TestKubePrismConfigV1Alpha1ConflictValidate(t *testing.T) {
	t.Parallel()

	cfg := kubePrismConfig()

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
			name: "legacy KubePrism present",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{ //nolint:staticcheck // testing legacy config conflict
						KubePrismSupport: &v1alpha1.KubePrism{}, //nolint:staticcheck // testing legacy config conflict
					},
				},
			},
			expectedError: "KubePrism config in v1alpha1 config (.machine.features.kubePrism) can't be used with KubePrismConfig document, please remove it to avoid conflicts",
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
