// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ipmi_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/ipmi"
)

const (
	// netfn/cmd pairs issued by the package, see IPMI spec 20.1 and 23.2.
	netfnApp             = 0x06
	netfnTransport       = 0x0c
	cmdGetDeviceID       = 0x01
	cmdGetLANConfigParam = 0x02

	// ccInvalidCommand is what a BMC answers for a command it doesn't implement.
	ccInvalidCommand = 0xc1
)

// fixtureTransport replays recorded BMC responses from testdata/<fixture>.
//
// Each file holds the response payload (everything after the completion code) as
// whitespace-separated hex bytes; a missing file stands for a command the BMC doesn't
// implement, answered with completion code 0xc1.
func fixtureTransport(t *testing.T, fixture string) ipmi.Transport {
	t.Helper()

	return func(_ context.Context, netfn, cmd byte, data []byte) (byte, []byte, error) {
		var name string

		switch {
		case netfn == netfnApp && cmd == cmdGetDeviceID:
			name = "device_id"
		case netfn == netfnTransport && cmd == cmdGetLANConfigParam:
			require.Len(t, data, 4)

			name = fmt.Sprintf("lan%d-param%d", data[0], data[1])
		default:
			return 0, nil, fmt.Errorf("unexpected request netfn 0x%02x cmd 0x%02x", netfn, cmd)
		}

		contents, err := os.ReadFile(filepath.Join("testdata", fixture, name))
		if errors.Is(err, os.ErrNotExist) {
			return ccInvalidCommand, nil, nil
		}

		require.NoError(t, err)

		resp, err := hex.DecodeString(strings.Join(strings.Fields(string(contents)), ""))
		require.NoError(t, err)

		return 0, resp, nil
	}
}

func TestDeviceID(t *testing.T) {
	for _, test := range []struct {
		fixture  string
		expected ipmi.DeviceInfo
		errored  bool
	}{
		{
			fixture: "dell-idrac",
			expected: ipmi.DeviceInfo{
				ManufacturerID: 674,
				ProductID:      0x0100,
				Firmware:       "7.10",
				IPMIVersion:    "2.0",
			},
		},
		{
			fixture: "supermicro-nolan",
			expected: ipmi.DeviceInfo{
				ManufacturerID: 47488,
				ProductID:      666,
				Firmware:       "3.05",
				IPMIVersion:    "2.0",
			},
		},
		{
			fixture: "hpe-ilo-partial",
			expected: ipmi.DeviceInfo{
				ManufacturerID: 47196,
				ProductID:      0x2220,
				Firmware:       "2.55",
				IPMIVersion:    "2.0",
			},
		},
		{
			// as emulated by `talosctl cluster create dev --with-ipmi`
			fixture: "qemu-bmc-sim",
			expected: ipmi.DeviceInfo{
				ManufacturerID: 674,
				ProductID:      666,
				Firmware:       "7.10",
				IPMIVersion:    "2.0",
			},
		},
		{
			// the upper nibble of the last manufacturer id byte is reserved, and
			// the IPMI version is BCD with the minor digit in the upper nibble
			fixture: "reserved-mfg-nibble",
			expected: ipmi.DeviceInfo{
				ManufacturerID: 674,
				ProductID:      666,
				Firmware:       "1.05",
				IPMIVersion:    "1.5",
			},
		},
		{
			fixture: "short-device-id",
			errored: true,
		},
	} {
		t.Run(test.fixture, func(t *testing.T) {
			info, err := ipmi.DeviceID(t.Context(), fixtureTransport(t, test.fixture))

			if test.errored {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, info)
		})
	}
}

func TestFindLANConfig(t *testing.T) {
	for _, test := range []struct {
		name     string
		fixture  string
		expected ipmi.LANConfig
		errored  bool
	}{
		{
			name:    "fully configured",
			fixture: "dell-idrac",
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
			name:    "address only, non-default channel",
			fixture: "hpe-ilo-partial",
			expected: ipmi.LANConfig{
				Channel: 2,
				Address: netip.MustParsePrefix("192.168.1.9/32"),
			},
		},
		{
			// a non-contiguous netmask and a truncated MAC are both dropped
			// rather than failing the whole channel
			name:    "unusable netmask and MAC",
			fixture: "bad-netmask",
			expected: ipmi.LANConfig{
				Channel: 1,
				Address: netip.MustParsePrefix("10.0.5.7/32"),
			},
		},
		{
			name:    "channel with no address configured",
			fixture: "supermicro-nolan",
			errored: true,
		},
		{
			name:    "truncated address payload",
			fixture: "short-lan-params",
			errored: true,
		},
		{
			// the QEMU BMC simulator implements no transport netfn commands at all
			name:    "no LAN commands implemented",
			fixture: "qemu-bmc-sim",
			errored: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := ipmi.FindLANConfig(t.Context(), fixtureTransport(t, test.fixture))

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
// instead of being flattened into ErrNoLANChannel: the fixture would otherwise succeed.
func TestFindLANConfigCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := ipmi.FindLANConfig(ctx, fixtureTransport(t, "dell-idrac"))
	assert.ErrorIs(t, err, context.Canceled)
}

func TestVendor(t *testing.T) {
	assert.Equal(t, "Dell", ipmi.Vendor(674))
	assert.Equal(t, "Supermicro", ipmi.Vendor(47488))
	assert.Empty(t, ipmi.Vendor(1))
}
