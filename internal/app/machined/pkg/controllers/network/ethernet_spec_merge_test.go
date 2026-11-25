// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestMergeEthernetSpecs(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		specs    map[resource.ID]network.EthernetSpecSpec
		expected map[resource.ID]*network.EthernetSpecSpec
	}{
		{
			name: "name overrides selector",
			specs: map[resource.ID]network.EthernetSpecSpec{
				"selector/eth0": {LinkName: "eth0", FromSelector: true, Rings: network.EthernetRingsSpec{RX: new(uint32(256)), TX: new(uint32(256))}},
				"net0":          {LinkName: "eth0", Rings: network.EthernetRingsSpec{RX: new(uint32(1024))}}, // matched by alias
			},
			expected: map[resource.ID]*network.EthernetSpecSpec{
				"eth0": {LinkName: "eth0", Rings: network.EthernetRingsSpec{RX: new(uint32(1024)), TX: new(uint32(256))}},
			},
		},
		{
			name: "exact name overrides alias",
			specs: map[resource.ID]network.EthernetSpecSpec{
				"selector/eth0": {LinkName: "eth0", FromSelector: true, Rings: network.EthernetRingsSpec{RX: new(uint32(256)), TX: new(uint32(256))}},
				"net0":          {LinkName: "eth0", Rings: network.EthernetRingsSpec{RX: new(uint32(512)), TX: new(uint32(512))}},
				"eth0":          {LinkName: "eth0", Rings: network.EthernetRingsSpec{RX: new(uint32(1024))}},
			},
			expected: map[resource.ID]*network.EthernetSpecSpec{
				"eth0": {LinkName: "eth0", Rings: network.EthernetRingsSpec{RX: new(uint32(1024)), TX: new(uint32(512))}},
			},
		},
		{
			name: "selectors merge",
			specs: map[resource.ID]network.EthernetSpecSpec{
				"rings/eth0":    {LinkName: "eth0", FromSelector: true, Rings: network.EthernetRingsSpec{RX: new(uint32(256))}},
				"channels/eth0": {LinkName: "eth0", FromSelector: true, Channels: network.EthernetChannelsSpec{Combined: new(uint32(4))}},
				"rings/eth1":    {LinkName: "eth1", FromSelector: true, Rings: network.EthernetRingsSpec{RX: new(uint32(256))}},
			},
			expected: map[resource.ID]*network.EthernetSpecSpec{
				"eth0": {
					LinkName: "eth0",
					Rings:    network.EthernetRingsSpec{RX: new(uint32(256))},
					Channels: network.EthernetChannelsSpec{Combined: new(uint32(4))},
				},
				"eth1": {LinkName: "eth1", Rings: network.EthernetRingsSpec{RX: new(uint32(256))}},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var list resource.List

			for id, spec := range test.specs {
				res := network.NewEthernetSpec(network.ConfigNamespaceName, id)
				*res.TypedSpec() = spec

				list.Items = append(list.Items, res)
			}

			assert.Equal(t, test.expected, netctrl.MergeEthernetSpecsForTest(zap.NewNop(), safe.NewList[*network.EthernetSpec](list)))
		})
	}
}
