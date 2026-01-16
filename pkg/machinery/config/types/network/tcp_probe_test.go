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
)

//go:embed testdata/tcpprobeconfig.yaml
var expectedTCPProbeConfigDocument []byte

func TestTCPProbeConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := network.NewTCPProbeConfigV1Alpha1("proxy-check")
	cfg.CommonProbeConfig = network.CommonProbeConfig{
		ProbeInterval:         time.Second,
		ProbeFailureThreshold: 3,
	}
	cfg.TCPEndpoint = "proxy.example.com:3128"
	cfg.TCPTimeout = 10 * time.Second

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedTCPProbeConfigDocument, marshaled)
}

func TestTCPProbeConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedTCPProbeConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.TCPProbeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.TCPProbeKind,
		},
		MetaName: "proxy-check",
		CommonProbeConfig: network.CommonProbeConfig{
			ProbeInterval:         time.Second,
			ProbeFailureThreshold: 3,
		},
		TCPEndpoint: "proxy.example.com:3128",
		TCPTimeout:  10 * time.Second,
	}, docs[0])
}

func TestTCPProbeConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.TCPProbeConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "valid config",
			cfg: func() *network.TCPProbeConfigV1Alpha1 {
				c := network.NewTCPProbeConfigV1Alpha1("test-probe")
				c.CommonProbeConfig = network.CommonProbeConfig{
					ProbeInterval:         time.Second,
					ProbeFailureThreshold: 3,
				}
				c.TCPEndpoint = "example.com:80"
				c.TCPTimeout = 5 * time.Second

				return c
			},
		},
		{
			name: "missing name",
			cfg: func() *network.TCPProbeConfigV1Alpha1 {
				c := network.NewTCPProbeConfigV1Alpha1("")
				c.TCPEndpoint = "example.com:80"

				return c
			},
			expectedError: "probe name is required",
		},
		{
			name: "missing TCP endpoint",
			cfg: func() *network.TCPProbeConfigV1Alpha1 {
				c := network.NewTCPProbeConfigV1Alpha1("probe44")

				return c
			},
			expectedError: "TCP probe endpoint is required",
		},
		{
			name: "negative values",
			cfg: func() *network.TCPProbeConfigV1Alpha1 {
				c := network.NewTCPProbeConfigV1Alpha1("probe33")
				c.CommonProbeConfig.ProbeFailureThreshold = -1
				c.CommonProbeConfig.ProbeInterval = -time.Second
				c.TCPTimeout = -5 * time.Second
				c.TCPEndpoint = "example.com:443"

				return c
			},
			expectedError: "TCP probe timeout cannot be negative: -5s\nprobe interval cannot be negative: -1s\nprobe failure threshold cannot be negative: -1",
		},
		{
			name: "empty",
			cfg: func() *network.TCPProbeConfigV1Alpha1 {
				return network.NewTCPProbeConfigV1Alpha1("")
			},
			expectedError: "probe name is required\nTCP probe endpoint is required",
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
