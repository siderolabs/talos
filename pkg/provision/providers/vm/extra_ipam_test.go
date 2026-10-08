// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

func TestExtraGuestReservationsInheritNetworkDNS(t *testing.T) {
	statePath := t.TempDir()
	network := provision.NetworkRequest{
		MTU: 1450,
		Nameservers: []netip.Addr{
			netip.MustParseAddr("192.0.2.53"),
			netip.MustParseAddr("2001:db8::53"),
		},
		ExtraDHCPRecords: []provision.DHCPRecord{
			{
				IP:      netip.MustParsePrefix("10.5.0.100/24"),
				MAC:     "52:54:00:00:01:01",
				Gateway: netip.MustParseAddr("10.5.0.1"),
				Name:    "extra-v4",
			},
			{
				IP:      netip.MustParsePrefix("2001:db8:1::100/64"),
				MAC:     "52:54:00:00:02:02",
				Gateway: netip.MustParseAddr("2001:db8:1::1"),
				Name:    "extra-v6",
			},
		},
	}
	require.NoError(t, vm.DumpExtraIPAMRecords(statePath, network))

	records, err := vm.LoadIPAMRecords(statePath)
	require.NoError(t, err)

	v4 := records[network.ExtraDHCPRecords[0].MAC][4]
	v6 := records[network.ExtraDHCPRecords[1].MAC][6]
	require.Equal(t, []netip.Addr{network.Nameservers[0]}, v4.Nameservers)
	require.Equal(t, []netip.Addr{network.Nameservers[1]}, v6.Nameservers)
	require.Equal(t, network.ExtraDHCPRecords[0].IP.Addr(), v4.IP)
	require.Equal(t, network.ExtraDHCPRecords[0].Gateway, v4.Gateway)
	require.Equal(t, network.MTU, v4.MTU)
	require.Equal(t, byte(24), v4.Netmask)
	require.Equal(t, "extra-v4", v4.Hostname)
	require.Empty(t, v4.TFTPServer)
	require.Empty(t, v4.IPXEBootFilename)
}

func TestExtraGuestReservationsReportWriteFailure(t *testing.T) {
	network := provision.NetworkRequest{
		ExtraDHCPRecords: []provision.DHCPRecord{
			{
				IP:  netip.MustParsePrefix("10.5.0.100/24"),
				MAC: "52:54:00:00:01:01",
			},
		},
	}
	require.Error(t, vm.DumpExtraIPAMRecords(t.TempDir()+"/missing", network))
}
