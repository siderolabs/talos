// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"iter"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestLinkNameResolver(t *testing.T) {
	t.Parallel()

	link1 := network.NewLinkStatus(network.NamespaceName, "eth0")
	link1.TypedSpec().Alias = "net0"
	link1.TypedSpec().AltNames = []string{"ext0"}

	link2 := network.NewLinkStatus(network.NamespaceName, "eth1")

	link3 := network.NewLinkStatus(network.NamespaceName, "eth2")
	link3.TypedSpec().AltNames = []string{"ext2"}

	links := []*network.LinkStatus{
		link1,
		link2,
		link3,
	}

	resolver := network.NewLinkResolver(func() iter.Seq[*network.LinkStatus] { return slices.Values(links) })

	assert.Equal(t, "eth0", resolver.Resolve("eth0"))
	assert.Equal(t, "eth0", resolver.Resolve("net0"))
	assert.Equal(t, "eth0", resolver.Resolve("ext0"))
	assert.Equal(t, "eth1", resolver.Resolve("eth1"))
	assert.Equal(t, "eth2", resolver.Resolve("eth2"))
	assert.Equal(t, "eth2", resolver.Resolve("ext2"))
	assert.Equal(t, "eth3", resolver.Resolve("eth3"))
}
