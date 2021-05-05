// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package wglan

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"inet.af/netaddr"
)

type NetworkdSuite struct {
	suite.Suite
}

func TestNetworkdSuite(t *testing.T) {
	suite.Run(t, new(NetworkdSuite))
}

func (suite *NetworkdSuite) TestMatchExistingSubnet() {
	type testStruct struct {
		IP       netaddr.IP
		Existing []netaddr.IPPrefix
		Bits     uint8
	}

	tests := []testStruct{

		// Normal matching IPv4
		{
			IP: netaddr.MustParseIP("192.168.1.12"),
			Existing: []netaddr.IPPrefix{
				netaddr.MustParseIPPrefix("2001:db8::d00d/64"),
				netaddr.MustParseIPPrefix("192.168.5.10/26"),
				netaddr.MustParseIPPrefix("192.168.1.1/24"),
			},
			Bits: 24,
		},

		// Normal matching IPv6
		{
			IP: netaddr.MustParseIP("2001:db8:0010::10"),
			Existing: []netaddr.IPPrefix{
				netaddr.MustParseIPPrefix("2001:db8::d00d/64"),
				netaddr.MustParseIPPrefix("192.168.5.10/26"),
				netaddr.MustParseIPPrefix("192.168.1.1/24"),
				netaddr.MustParseIPPrefix("2001:db8:0010::1/48"),
			},
			Bits: 48,
		},

		// Empty match set
		{
			IP:       netaddr.MustParseIP("192.168.1.12"),
			Existing: nil,
			Bits:     32,
		},

		// Out of bounds IPv6
		{
			IP: netaddr.MustParseIP("2001:db8:0010::10"),
			Existing: []netaddr.IPPrefix{
				netaddr.MustParseIPPrefix("2001:db8::d00d/64"),
				netaddr.MustParseIPPrefix("192.168.5.10/26"),
				netaddr.MustParseIPPrefix("192.168.1.1/24"),
				netaddr.MustParseIPPrefix("2001:db8:0011::1/48"),
			},
			Bits: 128,
		},
	}

	for i, ts := range tests {
		out := SubnetBitsMatch(ts.IP, ts.Existing)
		suite.Assert().Equalf(ts.Bits, out, "failed to match test index %d %q", i, ts.IP.String())
	}
}
