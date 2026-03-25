// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/timesyncconfig.yaml
var expectedTimeSyncConfigDocument []byte

//go:embed testdata/timesyncconfig_nts.yaml
var expectedTimeSyncConfigNTSDocument []byte

func TestTimeSyncConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewTimeSyncConfigV1Alpha1()
	cfg.TimeEnabled = new(true)
	cfg.TimeBootTimeout = time.Minute
	cfg.TimeNTP = &network.NTPConfig{
		Servers: []string{"time.cloudflare.com"},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedTimeSyncConfigDocument, marshaled)
}

func TestTimeSyncConfigMarshalStabilityNTS(t *testing.T) {
	t.Parallel()

	cfg := network.NewTimeSyncConfigV1Alpha1()
	cfg.TimeNTP = &network.NTPConfig{
		Servers: []string{"time.cloudflare.com"},
		UseNTS:  new(true),
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedTimeSyncConfigNTSDocument, marshaled)
}

func TestTimeSyncConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedTimeSyncConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.TimeSyncConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.TimeSyncKind,
		},
		TimeEnabled:     new(true),
		TimeBootTimeout: time.Minute,
		TimeNTP: &network.NTPConfig{
			Servers: []string{"time.cloudflare.com"},
		},
	}, docs[0])
}

func TestTimeSyncConfigUnmarshalNTS(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedTimeSyncConfigNTSDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	cfg, ok := docs[0].(*network.TimeSyncConfigV1Alpha1)
	require.True(t, ok)

	assert.True(t, cfg.UseNTS())
	assert.Equal(t, []string{"time.cloudflare.com"}, cfg.Servers())
}

func TestTimeSyncValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.TimeSyncConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  network.NewTimeSyncConfigV1Alpha1,
		},
		{
			name: "both NTP and PTP set",
			cfg: func() *network.TimeSyncConfigV1Alpha1 {
				cfg := network.NewTimeSyncConfigV1Alpha1()
				cfg.TimeNTP = &network.NTPConfig{
					Servers: []string{"pool.ntp.org"},
				}
				cfg.TimePTP = &network.PTPConfig{
					Devices: []string{"/dev/ptp0"},
				}

				return cfg
			},
			expectedError: "only one of ntp or ptp configuration can be specified",
		},
		{
			name: "negative boot timeout",
			cfg: func() *network.TimeSyncConfigV1Alpha1 {
				cfg := network.NewTimeSyncConfigV1Alpha1()
				cfg.TimeBootTimeout = -time.Second

				return cfg
			},
			expectedError: "bootTimeout cannot be negative",
		},
		{
			name: "valid NTP config",
			cfg: func() *network.TimeSyncConfigV1Alpha1 {
				cfg := network.NewTimeSyncConfigV1Alpha1()
				cfg.TimeNTP = &network.NTPConfig{
					Servers: []string{"pool.ntp.org"},
				}
				cfg.TimeBootTimeout = time.Second

				return cfg
			},
		},
		{
			name: "valid NTP config with NTS",
			cfg: func() *network.TimeSyncConfigV1Alpha1 {
				cfg := network.NewTimeSyncConfigV1Alpha1()
				cfg.TimeNTP = &network.NTPConfig{
					Servers: []string{"time.cloudflare.com"},
					UseNTS:  new(true),
				}

				return cfg
			},
		},
		{
			name: "NTS with IP address is invalid",
			cfg: func() *network.TimeSyncConfigV1Alpha1 {
				cfg := network.NewTimeSyncConfigV1Alpha1()
				cfg.TimeNTP = &network.NTPConfig{
					Servers: []string{"192.0.2.1"},
					UseNTS:  new(true),
				}

				return cfg
			},
			expectedError: `NTS requires hostnames, not IP addresses: "192.0.2.1"`,
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

func TestTimeSyncV1Alpha1Validate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		v1alpha1Cfg *v1alpha1.Config
		cfg         func() *network.TimeSyncConfigV1Alpha1

		expectedError string
	}{
		{
			name:        "empty",
			v1alpha1Cfg: &v1alpha1.Config{},
			cfg:         network.NewTimeSyncConfigV1Alpha1,
		},
		{
			name: "v1alpha1 timeservers set",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineTime: &v1alpha1.TimeConfig{
						TimeServers: []string{"za.pool.ntp.org"},
					},
				},
			},
			cfg: network.NewTimeSyncConfigV1Alpha1,

			expectedError: "time servers cannot be specified in both v1alpha1 and new-style configuration",
		},
		{
			name: "v1alpha1 boot timeout set",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineTime: &v1alpha1.TimeConfig{
						TimeBootTimeout: time.Second,
					},
				},
			},
			cfg: network.NewTimeSyncConfigV1Alpha1,

			expectedError: "boot timeout cannot be specified in both v1alpha1 and new-style configuration",
		},
		{
			name: "v1alpha1 disable set",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineTime: &v1alpha1.TimeConfig{
						TimeDisabled: new(true),
					},
				},
			},
			cfg: network.NewTimeSyncConfigV1Alpha1,

			expectedError: "time sync cannot be disabled in both v1alpha1 and new-style configuration",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.cfg().V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestTimeSyncUseNTS(t *testing.T) {
	t.Parallel()

	t.Run("nil NTP config returns false", func(t *testing.T) {
		t.Parallel()

		cfg := network.NewTimeSyncConfigV1Alpha1()
		assert.False(t, cfg.UseNTS())
	})

	t.Run("PTP config returns false", func(t *testing.T) {
		t.Parallel()

		cfg := network.NewTimeSyncConfigV1Alpha1()
		cfg.TimePTP = &network.PTPConfig{
			Devices: []string{"/dev/ptp0"},
		}

		assert.False(t, cfg.UseNTS())
	})

	t.Run("NTP config without NTS returns false", func(t *testing.T) {
		t.Parallel()

		cfg := network.NewTimeSyncConfigV1Alpha1()
		cfg.TimeNTP = &network.NTPConfig{
			Servers: []string{"pool.ntp.org"},
		}

		assert.False(t, cfg.UseNTS())
	})

	t.Run("NTP config with NTS returns true", func(t *testing.T) {
		t.Parallel()

		cfg := network.NewTimeSyncConfigV1Alpha1()
		cfg.TimeNTP = &network.NTPConfig{
			Servers: []string{"time.cloudflare.com"},
			UseNTS:  new(true),
		}

		assert.True(t, cfg.UseNTS())
	})

	t.Run("NTP config with NTS explicitly false returns false", func(t *testing.T) {
		t.Parallel()

		cfg := network.NewTimeSyncConfigV1Alpha1()
		cfg.TimeNTP = &network.NTPConfig{
			Servers: []string{"pool.ntp.org"},
			UseNTS:  new(false),
		}

		assert.False(t, cfg.UseNTS())
	})
}
