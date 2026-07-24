// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/provision/providers/qemu"
)

func TestFabricDevice(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		"e1000,netdev=fabric0,mac=02:00:00:00:00:01",
		qemu.FabricDeviceForTest(false, 0, "02:00:00:00:00:01", 0),
	)

	assert.Equal(t,
		"virtio-net-pci,netdev=fabric1,mac=02:00:00:00:00:01,addr=0x11,host_mtu=1430",
		qemu.FabricDeviceForTest(true, 1, "02:00:00:00:00:01", 1430),
	)
}

func TestBuildFabricUplinks(t *testing.T) {
	t.Parallel()

	numbered := qemu.BuildFabricUplinksForTest("talos-default", "talos1234", 0, 0, 1500, true, false)
	require.Len(t, numbered, 1)
	assert.Equal(t, "talos1234", numbered[0].BridgeName)
	assert.Equal(t, "vethvrf", numbered[0].IfName)
	assert.Contains(t, numbered[0].CNIConfList, `"bridge":"talos1234"`)

	clos := qemu.BuildFabricUplinksForTest("talos-default", "talos1234", 0, 2, 1430, true, true)
	require.Len(t, clos, 2)
	assert.NotEqual(t, "talos1234", clos[0].BridgeName)
	assert.NotEqual(t, "talos1234", clos[1].BridgeName)

	assert.Empty(t, qemu.BuildFabricUplinksForTest("talos-default", "talos1234", 0, 0, 1500, false, false))
}
