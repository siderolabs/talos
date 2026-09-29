// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ipmi_test

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"

	"github.com/bougou/go-ipmi/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/ipmi"
)

// channelParams is the Get LAN Configuration Parameters response payload per channel
// and parameter selector.
type channelParams map[uint8]map[types.LanConfigParamSelector][]byte

// getter replays the recorded payloads: a selector a channel has no entry for stands
// for a command the BMC doesn't implement.
func (p channelParams) getter() ipmi.LANParamGetter {
	return func(_ context.Context, channel uint8, param types.LanConfigParameter) error {
		selector, _, _ := param.LanConfigParameter()

		data, ok := p[channel][selector]
		if !ok {
			return fmt.Errorf("channel %d does not implement param %d", channel, selector)
		}

		return param.Unpack(data)
	}
}

func TestFindLANConfig(t *testing.T) {
	for _, test := range []struct {
		name     string
		params   channelParams
		expected ipmi.LANConfig
		errored  bool
	}{
		{
			name: "fully configured",
			params: channelParams{
				1: {
					types.LanConfigParamSelector_IP:               {10, 0, 5, 7},
					types.LanConfigParamSelector_SubnetMask:       {255, 255, 255, 0},
					types.LanConfigParamSelector_MAC:              {0x4c, 0xd9, 0x8f, 0x01, 0x02, 0x03},
					types.LanConfigParamSelector_DefaultGatewayIP: {10, 0, 5, 1},
				},
			},
			expected: ipmi.LANConfig{
				Channel:      1,
				Address:      netip.MustParsePrefix("10.0.5.7/24"),
				Gateway:      netip.MustParseAddr("10.0.5.1"),
				HardwareAddr: net.HardwareAddr{0x4c, 0xd9, 0x8f, 0x01, 0x02, 0x03},
			},
		},
		{
			// only the address selector is implemented, and only on channel 2:
			// the netmask falls back to a host prefix, gateway and MAC stay empty
			name: "address only, non-default channel",
			params: channelParams{
				2: {
					types.LanConfigParamSelector_IP: {192, 168, 1, 9},
				},
			},
			expected: ipmi.LANConfig{
				Channel: 2,
				Address: netip.MustParsePrefix("192.168.1.9/32"),
			},
		},
		{
			// a non-contiguous netmask and a truncated MAC are both dropped
			// rather than failing the whole channel
			name: "unusable netmask and MAC",
			params: channelParams{
				1: {
					types.LanConfigParamSelector_IP:         {10, 0, 5, 7},
					types.LanConfigParamSelector_SubnetMask: {255, 0, 255, 0},
					types.LanConfigParamSelector_MAC:        {0x4c, 0xd9, 0x8f},
				},
			},
			expected: ipmi.LANConfig{
				Channel: 1,
				Address: netip.MustParsePrefix("10.0.5.7/32"),
			},
		},
		{
			name: "channel with no address configured",
			params: channelParams{
				1: {
					types.LanConfigParamSelector_IP: {0, 0, 0, 0},
				},
			},
			errored: true,
		},
		{
			name: "truncated address payload",
			params: channelParams{
				1: {
					types.LanConfigParamSelector_IP: {10, 0},
				},
			},
			errored: true,
		},
		{
			// the QEMU BMC simulator implements no transport netfn commands at all
			name:    "no LAN commands implemented",
			params:  channelParams{},
			errored: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := ipmi.FindLANConfig(t.Context(), test.params.getter())

			if test.errored {
				assert.ErrorIs(t, err, ipmi.ErrNoLANChannel)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, cfg)
		})
	}
}

// TestFindLANConfigCancelled asserts that a canceled context aborts the channel probe
// instead of being flattened into ErrNoLANChannel: the params would otherwise succeed.
func TestFindLANConfigCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	params := channelParams{1: {types.LanConfigParamSelector_IP: {10, 0, 5, 7}}}

	_, err := ipmi.FindLANConfig(ctx, params.getter())
	assert.ErrorIs(t, err, context.Canceled)
}

func TestDeviceNumber(t *testing.T) {
	for path, expected := range map[string]int32{
		"/dev/ipmi0":  0,
		"/dev/ipmi12": 12,
	} {
		num, ok := ipmi.DeviceNumber(path)

		assert.True(t, ok, path)
		assert.Equal(t, expected, num, path)
	}

	// the glob also matches the directory some udev rulesets create
	_, ok := ipmi.DeviceNumber("/dev/ipmi")
	assert.False(t, ok)
}

func TestVendor(t *testing.T) {
	assert.Equal(t, "Dell", ipmi.Vendor(674))
	assert.Equal(t, "Supermicro", ipmi.Vendor(47488))
	assert.Empty(t, ipmi.Vendor(1))
}
