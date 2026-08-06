// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestBGPStatusSpecProjection(t *testing.T) {
	t.Parallel()

	link := network.NewLinkStatus(network.NamespaceName, "eth0")
	*link.TypedSpec() = network.LinkStatusSpec{
		Alias:       "fabric0",
		AltNames:    []string{"enp1s0"},
		Index:       10,
		MasterIndex: 11,
		Kind:        "veth",
	}

	address := network.NewAddressStatus(network.NamespaceName, "eth0/192.0.2.1/32")
	*address.TypedSpec() = network.AddressStatusSpec{
		Address:   netip.MustParsePrefix("192.0.2.1/32"),
		LinkName:  "eth0",
		LinkIndex: 10,
	}

	assert.Equal(
		t,
		map[string]network.LinkStatusSpec{"eth0": *link.TypedSpec()},
		netctrl.BGPLinkStatusSpecsForTest(slices.Values([]*network.LinkStatus{link})),
	)
	assert.Equal(
		t,
		[]network.AddressStatusSpec{*address.TypedSpec()},
		netctrl.BGPAddressSpecsForTest(slices.Values([]*network.AddressStatus{address})),
	)
}
