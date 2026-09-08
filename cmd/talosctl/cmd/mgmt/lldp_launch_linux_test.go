// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt_test

import (
	"testing"

	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt"
)

func TestLLDPBridgeIndex(t *testing.T) {
	links := []rtnetlink.LinkMessage{
		{},
		{Index: 9, Attributes: &rtnetlink.LinkAttributes{Name: "no-info"}},
		{Index: 10, Attributes: &rtnetlink.LinkAttributes{Name: "test-bridge", Info: &rtnetlink.LinkInfo{Kind: "bridge"}}},
		{Index: 11, Attributes: &rtnetlink.LinkAttributes{Name: "test-port", Info: &rtnetlink.LinkInfo{Kind: "veth"}}},
	}

	index, err := mgmt.LLDPBridgeIndexForTest(links, "test-bridge")
	assert.NoError(t, err)
	assert.EqualValues(t, 10, index)

	for _, name := range []string{"missing", "no-info", "test-port"} {
		_, err = mgmt.LLDPBridgeIndexForTest(links, name)
		assert.Error(t, err, "interface %q must not be accepted as the bridge", name)
	}
}

func TestLLDPBridgePort(t *testing.T) {
	master := uint32(10)
	otherMaster := uint32(11)

	for _, test := range []struct {
		name string
		link rtnetlink.LinkMessage
		want bool
	}{
		{name: "bridge veth", link: rtnetlink.LinkMessage{Attributes: &rtnetlink.LinkAttributes{Info: &rtnetlink.LinkInfo{Kind: "veth"}, Master: &master}}, want: true},
		{name: "other bridge", link: rtnetlink.LinkMessage{Attributes: &rtnetlink.LinkAttributes{Info: &rtnetlink.LinkInfo{Kind: "veth"}, Master: &otherMaster}}},
		{name: "no master", link: rtnetlink.LinkMessage{Attributes: &rtnetlink.LinkAttributes{Info: &rtnetlink.LinkInfo{Kind: "veth"}}}},
		{name: "no attributes", link: rtnetlink.LinkMessage{}},
		{name: "no info", link: rtnetlink.LinkMessage{Attributes: &rtnetlink.LinkAttributes{Master: &master}}},
		{name: "bridge itself", link: rtnetlink.LinkMessage{Index: master, Attributes: &rtnetlink.LinkAttributes{Info: &rtnetlink.LinkInfo{Kind: "bridge"}}}},
		{name: "non-veth port", link: rtnetlink.LinkMessage{Attributes: &rtnetlink.LinkAttributes{Info: &rtnetlink.LinkInfo{Kind: "dummy"}, Master: &master}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, mgmt.LLDPBridgePortForTest(test.link, master))
		})
	}
}
