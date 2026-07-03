// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package blockhelpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/types/block/blockhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func disk(devPath string) *block.Disk {
	d := block.NewDisk(block.NamespaceName, devPath)
	d.TypedSpec().DevPath = devPath

	return d
}

func volume(id, devPath, parentDevPath string) *block.DiscoveredVolume {
	v := block.NewDiscoveredVolume(block.NamespaceName, id)
	v.TypedSpec().DevPath = devPath
	v.TypedSpec().ParentDevPath = parentDevPath

	return v
}

func TestBuildMatchContexts(t *testing.T) {
	disks := []*block.Disk{disk("/dev/vda"), disk("/dev/vdb"), disk("/dev/vdc")}

	volumes := []*block.DiscoveredVolume{
		volume("vda", "/dev/vda", ""), // system disk, partitioned
		volume("vda1", "/dev/vda1", "/dev/vda"),
		volume("vdb", "/dev/vdb", ""), // whole data disk
		volume("vdc", "/dev/vdc", ""), // data disk carrying a partition
		volume("vdc1", "/dev/vdc1", "/dev/vdc"),
	}

	got, err := blockhelpers.BuildMatchContexts(disks, volumes, "/dev/vda")
	require.NoError(t, err)

	byPath := map[string]blockhelpers.MatchContext{}
	for _, c := range got {
		byPath[c.DevPath] = c
	}

	// System disk and its partition are flagged.
	assert.True(t, byPath["/dev/vda"].SystemDisk)
	assert.True(t, byPath["/dev/vda"].Partitioned)
	assert.True(t, byPath["/dev/vda1"].SystemDisk)

	// Whole data disk: usable, not partitioned, not system.
	assert.True(t, byPath["/dev/vdb"].Disk)
	assert.False(t, byPath["/dev/vdb"].Partitioned)
	assert.False(t, byPath["/dev/vdb"].SystemDisk)

	// Partitioned data disk is flagged busy, but its partition is a candidate.
	assert.True(t, byPath["/dev/vdc"].Partitioned)
	assert.False(t, byPath["/dev/vdc1"].Disk)
	assert.False(t, byPath["/dev/vdc1"].Partitioned)
	assert.False(t, byPath["/dev/vdc1"].SystemDisk)

	// Every context binds volume, disk and system_disk for CEL.
	for _, c := range got {
		assert.Contains(t, c.CELContext, "volume")
		assert.Contains(t, c.CELContext, "disk")
		assert.Contains(t, c.CELContext, "system_disk")
	}
}
