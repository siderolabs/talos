// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"net"
)

// FabricDeviceForTest exposes fabric device argument construction for tests.
func FabricDeviceForTest(clos bool, index int, mac string, mtu int) string {
	return fabricDevice(&LaunchConfig{
		CLOSNoNet0: clos,
		Network:    networkConfig{networkConfigBase: networkConfigBase{MTU: mtu}},
	}, index, FabricUplink{mac: mac})
}

// APIPortAllocatorForTest exposes apiPortAllocator for tests.
type APIPortAllocatorForTest struct {
	allocator apiPortAllocator
}

// Allocate reserves an API port for tests.
func (allocator *APIPortAllocatorForTest) Allocate(ctx context.Context, host string) (*net.TCPAddr, error) {
	return allocator.allocator.allocate(ctx, host)
}

// BuildFabricUplinksForTest exposes BGP test uplink construction for tests.
func BuildFabricUplinksForTest(networkName, managementBridge string, nodeIdx, count, mtu int, bgpEnabled, bgpCLOS bool) []FabricUplink {
	return buildFabricUplinks(networkName, managementBridge, nodeIdx, count, mtu, bgpEnabled, bgpCLOS)
}
