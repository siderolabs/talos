// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/cluster"
)

func TestAssertNodes(t *testing.T) {
	for _, tt := range []struct {
		name          string
		expectedNodes []cluster.NodeInfo
		actualNodes   []cluster.NodeInfo

		expectedError string
	}{
		{
			name: "aws+discovery",
			expectedNodes: []cluster.NodeInfo{
				{
					IPs: []netip.Addr{netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("172.23.1.2")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("5.6.7.8"), netip.MustParseAddr("172.23.1.3")},
				},
			},
			actualNodes: []cluster.NodeInfo{
				{
					IPs: []netip.Addr{netip.MustParseAddr("5.6.7.8"), netip.MustParseAddr("172.23.1.3")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("172.23.1.2")},
				},
			},
		},
		{
			name: "aws+private",
			expectedNodes: []cluster.NodeInfo{
				{
					IPs: []netip.Addr{netip.MustParseAddr("172.23.1.2")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("172.23.1.3")},
				},
			},
			actualNodes: []cluster.NodeInfo{
				{
					IPs: []netip.Addr{netip.MustParseAddr("5.6.7.8"), netip.MustParseAddr("172.23.1.3")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("172.23.1.2")},
				},
			},
		},
		{
			name: "more internal IPs",
			expectedNodes: []cluster.NodeInfo{
				{
					IPs: []netip.Addr{netip.MustParseAddr("ff::1"), netip.MustParseAddr("172.23.1.3")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("ff::2"), netip.MustParseAddr("172.23.1.2")},
				},
			},
			actualNodes: []cluster.NodeInfo{
				{
					IPs: []netip.Addr{netip.MustParseAddr("172.23.1.2")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("172.23.1.3")},
				},
			},
		},
		{
			name: "extra node expected",
			expectedNodes: []cluster.NodeInfo{
				{
					IPs: []netip.Addr{netip.MustParseAddr("172.23.1.2")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("172.23.1.3")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("172.23.1.4")},
				},
			},
			actualNodes: []cluster.NodeInfo{
				{
					IPs: []netip.Addr{netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("172.23.1.2")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("5.6.7.8"), netip.MustParseAddr("172.23.1.3")},
				},
			},
			expectedError: `can't find expected node with IPs ["172.23.1.4"]`,
		},
		{
			name: "extra node actual",
			expectedNodes: []cluster.NodeInfo{
				{
					IPs: []netip.Addr{netip.MustParseAddr("172.23.1.2")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("172.23.1.3")},
				},
			},
			actualNodes: []cluster.NodeInfo{
				{
					IPs: []netip.Addr{netip.MustParseAddr("1.2.3.4"), netip.MustParseAddr("172.23.1.2")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("5.6.7.8"), netip.MustParseAddr("172.23.1.3")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("172.23.1.4")},
				},
				{
					IPs: []netip.Addr{netip.MustParseAddr("172.23.1.5"), netip.MustParseAddr("9.10.11.12")},
				},
			},
			expectedError: "unexpected nodes with IPs [\"9.10.11.12\" \"172.23.1.4\" \"172.23.1.5\"]",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := cluster.NodesMatch(tt.expectedNodes, tt.actualNodes)

			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedError)
			}
		})
	}
}
