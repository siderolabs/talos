// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/kubespanconfig.yaml
var expectedKubeSpanConfigDocument []byte

func TestKubeSpanConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewKubeSpanV1Alpha1()
	cfg.ConfigEnabled = pointer.To(true)
	cfg.ConfigAdvertiseKubernetesNetworks = pointer.To(false)
	cfg.ConfigAllowDownPeerBypass = pointer.To(false)
	cfg.ConfigMTU = pointer.To(uint32(1420))

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKubeSpanConfigDocument, marshaled)
}

func TestKubeSpanConfigUnmarshal(t *testing.T) {
	t.Parallel()

	cfg := network.NewKubeSpanV1Alpha1()
	cfg.ConfigEnabled = pointer.To(true)
	cfg.ConfigMTU = pointer.To(uint32(1500))
	cfg.ConfigFilters = &network.KubeSpanFiltersConfig{
		ConfigEndpoints: []string{"0.0.0.0/0", "!192.168.0.0/16"},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	// Verify interface methods work
	assert.True(t, cfg.Enabled())
	assert.Equal(t, uint32(1500), cfg.MTU())
	assert.Equal(t, []string{"0.0.0.0/0", "!192.168.0.0/16"}, cfg.Filters().Endpoints())
}

func TestKubeSpanConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.KubeSpanConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "valid default",
			cfg: func() *network.KubeSpanConfigV1Alpha1 {
				cfg := network.NewKubeSpanV1Alpha1()
				cfg.ConfigEnabled = pointer.To(true)

				return cfg
			},
		},
		{
			name: "valid with MTU",
			cfg: func() *network.KubeSpanConfigV1Alpha1 {
				cfg := network.NewKubeSpanV1Alpha1()
				cfg.ConfigEnabled = pointer.To(true)
				cfg.ConfigMTU = pointer.To(uint32(1420))

				return cfg
			},
		},
		{
			name: "MTU too low",
			cfg: func() *network.KubeSpanConfigV1Alpha1 {
				cfg := network.NewKubeSpanV1Alpha1()
				cfg.ConfigEnabled = pointer.To(true)
				cfg.ConfigMTU = pointer.To(uint32(1279))

				return cfg
			},

			expectedError: "kubespan link MTU must be at least 1280",
		},
		{
			name: "MTU at minimum",
			cfg: func() *network.KubeSpanConfigV1Alpha1 {
				cfg := network.NewKubeSpanV1Alpha1()
				cfg.ConfigEnabled = pointer.To(true)
				cfg.ConfigMTU = pointer.To(uint32(1280))

				return cfg
			},
		},
		{
			name: "with filters",
			cfg: func() *network.KubeSpanConfigV1Alpha1 {
				cfg := network.NewKubeSpanV1Alpha1()
				cfg.ConfigEnabled = pointer.To(true)
				cfg.ConfigFilters = &network.KubeSpanFiltersConfig{
					ConfigEndpoints: []string{"0.0.0.0/0", "!10.0.0.0/8"},
				}

				return cfg
			},
		},
		{
			name: "with invalid filters",
			cfg: func() *network.KubeSpanConfigV1Alpha1 {
				cfg := network.NewKubeSpanV1Alpha1()
				cfg.ConfigEnabled = pointer.To(true)
				cfg.ConfigFilters = &network.KubeSpanFiltersConfig{
					ConfigEndpoints: []string{"0.0.0.0/0", "!/8"},
				}

				return cfg
			},
			expectedError: `KubeSpan endpoint filer is not valid: "/8"`,
		},
		{
			name: "all options enabled",
			cfg: func() *network.KubeSpanConfigV1Alpha1 {
				cfg := network.NewKubeSpanV1Alpha1()
				cfg.ConfigEnabled = pointer.To(true)
				cfg.ConfigAdvertiseKubernetesNetworks = pointer.To(true)
				cfg.ConfigAllowDownPeerBypass = pointer.To(true)
				cfg.ConfigHarvestExtraEndpoints = pointer.To(true)
				cfg.ConfigMTU = pointer.To(uint32(1400))

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

func TestKubeSpanConfigConflictValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.KubeSpanConfigV1Alpha1
		v1   func() *v1alpha1.Config

		expectedError string
	}{
		{
			name: "no conflict when v1alpha1 empty",
			cfg: func() *network.KubeSpanConfigV1Alpha1 {
				cfg := network.NewKubeSpanV1Alpha1()
				cfg.ConfigEnabled = pointer.To(true)

				return cfg
			},
			v1: func() *v1alpha1.Config {
				return &v1alpha1.Config{}
			},
		},
		{
			name: "conflict when both set",
			cfg: func() *network.KubeSpanConfigV1Alpha1 {
				cfg := network.NewKubeSpanV1Alpha1()
				cfg.ConfigEnabled = pointer.To(true)

				return cfg
			},
			v1: func() *v1alpha1.Config {
				cfg := &v1alpha1.Config{}
				cfg.MachineConfig = &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{ //nolint:staticcheck // legacy config
						NetworkKubeSpan: &v1alpha1.NetworkKubeSpan{ //nolint:staticcheck // legacy config
							KubeSpanEnabled: pointer.To(true),
						},
					},
				}

				return cfg
			},

			expectedError: "kubespan is already configured in v1alpha1 machine.network.kubespan",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.cfg().V1Alpha1ConflictValidate(test.v1())

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestKubeSpanConfigInterface(t *testing.T) {
	t.Parallel()

	cfg := network.NewKubeSpanV1Alpha1()
	cfg.ConfigEnabled = pointer.To(true)
	cfg.ConfigAdvertiseKubernetesNetworks = pointer.To(true)
	cfg.ConfigAllowDownPeerBypass = pointer.To(false)
	cfg.ConfigHarvestExtraEndpoints = pointer.To(true)
	cfg.ConfigMTU = pointer.To(uint32(1380))
	cfg.ConfigFilters = &network.KubeSpanFiltersConfig{
		ConfigEndpoints: []string{"192.168.0.0/16"},
	}

	// Test interface methods
	assert.True(t, cfg.Enabled())
	assert.True(t, cfg.AdvertiseKubernetesNetworks())
	assert.True(t, cfg.ForceRouting())
	assert.True(t, cfg.HarvestExtraEndpoints())
	assert.Equal(t, uint32(1380), cfg.MTU())
	assert.NotNil(t, cfg.Filters())
	assert.Equal(t, []string{"192.168.0.0/16"}, cfg.Filters().Endpoints())
}
