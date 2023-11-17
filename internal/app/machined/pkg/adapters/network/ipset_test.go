// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go4.org/netipx"

	"github.com/siderolabs/talos/internal/app/machined/pkg/adapters/network"
)

func TestBuildIPSet(t *testing.T) {
	ipset, err := network.BuildIPSet(
		[]netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/8"),
			netip.MustParsePrefix("2001:db8::/32"),
		},
		[]netip.Prefix{
			netip.MustParsePrefix("10.4.0.0/16"),
		})
	require.NoError(t, err)

	assert.Equal(t,
		[]string{"10.0.0.0-10.3.255.255", "10.5.0.0-10.255.255.255", "2001:db8::-2001:db8:ffff:ffff:ffff:ffff:ffff:ffff"},
		xslices.Map(ipset.Ranges(), netipx.IPRange.String),
	)
}

func TestSplitIPSet(t *testing.T) {
	ipset, err := network.BuildIPSet(
		[]netip.Prefix{
			netip.MustParsePrefix("10.0.0.0/8"),
			netip.MustParsePrefix("2001:db8::/32"),
		},
		[]netip.Prefix{
			netip.MustParsePrefix("10.4.0.0/16"),
		})
	require.NoError(t, err)

	v4, v6 := network.SplitIPSet(ipset)

	assert.Equal(t,
		[]string{"10.0.0.0-10.3.255.255", "10.5.0.0-10.255.255.255"},
		xslices.Map(v4, netipx.IPRange.String),
	)

	assert.Equal(t,
		[]string{"2001:db8::-2001:db8:ffff:ffff:ffff:ffff:ffff:ffff"},
		xslices.Map(v6, netipx.IPRange.String),
	)
}
