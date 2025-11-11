// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

//go:embed testdata/hostnameconfig.yaml
var expectedHostnameConfigDocument []byte

func TestHostnameConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewHostnameConfigV1Alpha1()
	cfg.ConfigAuto = pointer.To(nethelpers.AutoHostnameKindStable)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedHostnameConfigDocument, marshaled)
}

func TestHostnameConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.HostnameConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  network.NewHostnameConfigV1Alpha1,

			expectedError: "either 'auto' or 'hostname' must be set",
		},
		{
			name: "both set",
			cfg: func() *network.HostnameConfigV1Alpha1 {
				cfg := network.NewHostnameConfigV1Alpha1()
				cfg.ConfigHostname = "example.org"
				cfg.ConfigAuto = pointer.To(nethelpers.AutoHostnameKindStable)

				return cfg
			},

			expectedError: "'auto' and 'hostname' cannot be set at the same time",
		},
		{
			name: "invalid auto",
			cfg: func() *network.HostnameConfigV1Alpha1 {
				cfg := network.NewHostnameConfigV1Alpha1()
				cfg.ConfigAuto = pointer.To(nethelpers.AutoHostnameKindAddr)

				return cfg
			},

			expectedError: "invalid value for 'auto': talos-addr",
		},
		{
			name: "too long hostname",
			cfg: func() *network.HostnameConfigV1Alpha1 {
				cfg := network.NewHostnameConfigV1Alpha1()
				cfg.ConfigHostname = strings.Repeat("a", 64)

				return cfg
			},

			expectedError: "invalid hostname \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"",
		},
		{
			name: "too long hostname",
			cfg: func() *network.HostnameConfigV1Alpha1 {
				cfg := network.NewHostnameConfigV1Alpha1()
				cfg.ConfigHostname = strings.Repeat("a", 64) + "." + strings.Repeat("b", 64)

				return cfg
			},

			expectedError: "invalid hostname \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"",
		},
		{
			name: "invalid fqdn",
			cfg: func() *network.HostnameConfigV1Alpha1 {
				cfg := network.NewHostnameConfigV1Alpha1()
				cfg.ConfigHostname = ".example.org"

				return cfg
			},

			expectedError: "invalid hostname \"\"",
		},
		{
			name: "fqdn too long",
			cfg: func() *network.HostnameConfigV1Alpha1 {
				cfg := network.NewHostnameConfigV1Alpha1()
				cfg.ConfigHostname = strings.Repeat(strings.Repeat("a", 63)+".", 5)

				return cfg
			},

			expectedError: "fqdn is too long: 320",
		},
		{
			name: "valid 1",
			cfg: func() *network.HostnameConfigV1Alpha1 {
				cfg := network.NewHostnameConfigV1Alpha1()
				cfg.ConfigHostname = "example.org"

				return cfg
			},
		},
		{
			name: "valid 2",
			cfg: func() *network.HostnameConfigV1Alpha1 {
				cfg := network.NewHostnameConfigV1Alpha1()
				cfg.ConfigAuto = pointer.To(nethelpers.AutoHostnameKindStable)

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

func TestHostnameV1Alpha1Validate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		v1alpha1Cfg *v1alpha1.Config
		cfg         func() *network.HostnameConfigV1Alpha1

		expectedError string
	}{
		{
			name:        "empty",
			v1alpha1Cfg: &v1alpha1.Config{},
			cfg:         network.NewHostnameConfigV1Alpha1,
		},
		{
			name: "v1alpha1 static hostname set",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineNetwork: &v1alpha1.NetworkConfig{
						NetworkHostname: "foo",
					},
				},
			},
			cfg: network.NewHostnameConfigV1Alpha1,

			expectedError: "static hostname is already set in v1alpha1 config",
		},
		{
			name: "v1alpha1 stable hostname set",
			v1alpha1Cfg: &v1alpha1.Config{
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						StableHostname: pointer.To(true),
					},
				},
			},
			cfg: network.NewHostnameConfigV1Alpha1,

			expectedError: "stable hostname is already set in v1alpha1 config",
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
