// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt"
)

func TestLLDPBridgePort(t *testing.T) {
	assert.True(t, mgmt.LLDPBridgePortForTest(&netlink.Veth{LinkAttrs: netlink.LinkAttrs{MasterIndex: 10}}, 10))
	assert.False(t, mgmt.LLDPBridgePortForTest(&netlink.Veth{LinkAttrs: netlink.LinkAttrs{MasterIndex: 11}}, 10))
	assert.False(t, mgmt.LLDPBridgePortForTest(&netlink.Veth{}, 10))
	assert.False(t, mgmt.LLDPBridgePortForTest(&netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Index: 10}}, 10))
	assert.False(t, mgmt.LLDPBridgePortForTest(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{MasterIndex: 10}}, 10))
}
