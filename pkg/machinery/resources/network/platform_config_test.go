// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestPlatformConfigEqual(t *testing.T) {
	t.Parallel()

	assert.True(t, (&network.PlatformConfigSpec{}).Equal(&network.PlatformConfigSpec{}))
	assert.True(t, (&network.PlatformConfigSpec{Addresses: []network.AddressSpecSpec{}}).Equal(&network.PlatformConfigSpec{}))
	assert.True(t, (&network.PlatformConfigSpec{Addresses: []network.AddressSpecSpec{
		{
			Address:  netip.MustParsePrefix("192.168.68.54/22"),
			LinkName: "eth0",
			Family:   nethelpers.FamilyInet4,
			Scope:    nethelpers.ScopeGlobal,
		},
	}}).Equal(&network.PlatformConfigSpec{Addresses: []network.AddressSpecSpec{
		{
			Address:  netip.MustParsePrefix("192.168.68.54/22"),
			LinkName: "eth0",
			Family:   nethelpers.FamilyInet4,
			Scope:    nethelpers.ScopeGlobal,
		},
	}}))

	assert.False(t, (&network.PlatformConfigSpec{}).Equal(nil))
	assert.False(t, (&network.PlatformConfigSpec{Addresses: []network.AddressSpecSpec{
		{
			Address:  netip.MustParsePrefix("192.168.68.1/22"),
			LinkName: "eth0",
			Family:   nethelpers.FamilyInet4,
			Scope:    nethelpers.ScopeGlobal,
		},
	}}).Equal(&network.PlatformConfigSpec{Addresses: []network.AddressSpecSpec{
		{
			Address:  netip.MustParsePrefix("192.168.68.2/22"),
			LinkName: "eth1",
			Family:   nethelpers.FamilyInet4,
			Scope:    nethelpers.ScopeGlobal,
		},
	}}))
}
