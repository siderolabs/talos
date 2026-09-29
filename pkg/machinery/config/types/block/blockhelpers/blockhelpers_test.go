// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package blockhelpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
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

func luksVolume(id, devPath, parentDevPath string) *block.DiscoveredVolume {
	v := volume(id, devPath, parentDevPath)
	v.TypedSpec().Name = "luks"

	return v
}

func volumeStatus(id, location, mountLocation string) *block.VolumeStatus {
	s := block.NewVolumeStatus(block.NamespaceName, id)
	s.TypedSpec().Location = location
	s.TypedSpec().MountLocation = mountLocation

	return s
}

func contextsByPath(t *testing.T, contexts []blockhelpers.MatchContext) map[string]blockhelpers.MatchContext {
	t.Helper()

	byPath := map[string]blockhelpers.MatchContext{}

	for _, c := range contexts {
		_, dup := byPath[c.DevPath]
		assert.False(t, dup, "duplicate context for %q", c.DevPath)

		byPath[c.DevPath] = c
	}

	return byPath
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

	got, err := blockhelpers.BuildMatchContexts(disks, volumes, nil, nil, "/dev/vda")
	require.NoError(t, err)

	byPath := contextsByPath(t, got)

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

	// Every context binds volume, disk, volume_id and system_disk for CEL.
	for _, c := range got {
		assert.Contains(t, c.CELContext, "volume")
		assert.Contains(t, c.CELContext, "disk")
		assert.Contains(t, c.CELContext, "volume_id")
		assert.Contains(t, c.CELContext, "system_disk")
	}

	// Nothing claims these devices, so none carries a volume id.
	for _, c := range got {
		assert.Empty(t, c.VolumeID)
		assert.Equal(t, "", c.CELContext["volume_id"])
	}
}

func TestBuildMatchContextsEncryptedVolume(t *testing.T) {
	// r-lvmdata is an encrypted raw volume on /dev/vdb1, opened as /dev/dm-0.
	disks := []*block.Disk{disk("/dev/vdb"), disk("/dev/dm-0")}

	volumes := []*block.DiscoveredVolume{
		volume("vdb", "/dev/vdb", ""),
		luksVolume("vdb1", "/dev/vdb1", "/dev/vdb"),
		volume("dm-0", "/dev/dm-0", ""),
	}

	statuses := []*block.VolumeStatus{volumeStatus("r-lvmdata", "/dev/vdb1", "/dev/dm-0")}

	got, err := blockhelpers.BuildMatchContexts(disks, volumes, statuses, nil, "")
	require.NoError(t, err)

	byPath := contextsByPath(t, got)

	// The ciphertext is never offered as a device of its own.
	assert.NotContains(t, byPath, "/dev/vdb1")

	// The opened device is offered under the ciphertext's identity: a selector
	// matching the partition label or the volume id lands on /dev/dm-0. The
	// anonymous /dev/dm-0 entry is deduped away, asserted by contextsByPath.
	opened := byPath["/dev/dm-0"]
	assert.Equal(t, "r-lvmdata", opened.VolumeID)
	assert.Equal(t, "r-lvmdata", opened.CELContext["volume_id"])
	assert.Equal(t, "/dev/vdb1", opened.CELContext["volume"].(*blockpb.DiscoveredVolumeSpec).DevPath)

	// A partition's `disk` stays unbound, as for any partition.
	assert.Empty(t, opened.CELContext["disk"].(*blockpb.DiskSpec).DevPath)
	assert.False(t, opened.Disk)
}

func TestBuildMatchContextsVolumeNotPrepared(t *testing.T) {
	disks := []*block.Disk{disk("/dev/vdb")}

	volumes := []*block.DiscoveredVolume{
		volume("vdb", "/dev/vdb", ""),
		volume("vdb1", "/dev/vdb1", "/dev/vdb"),
		volume("vdb2", "/dev/vdb2", "/dev/vdb"),
	}

	statuses := []*block.VolumeStatus{
		// located, but the volume manager has not prepared it: no MountLocation yet
		volumeStatus("r-pending", "/dev/vdb1", ""),
		volumeStatus("r-ready", "/dev/vdb2", "/dev/vdb2"),
	}

	got, err := blockhelpers.BuildMatchContexts(disks, volumes, statuses, nil, "")
	require.NoError(t, err)

	byPath := contextsByPath(t, got)

	assert.NotContains(t, byPath, "/dev/vdb1")

	// An unencrypted volume takes the same path, where MountLocation == Location.
	assert.Equal(t, "/dev/vdb2", byPath["/dev/vdb2"].DevPath)
	assert.Equal(t, "r-ready", byPath["/dev/vdb2"].VolumeID)
}

func TestBuildMatchContextsUnclaimedCiphertext(t *testing.T) {
	// A LUKS device no volume claims: a foreign volume, or one whose status has
	// not been published yet. Either way it must not be handed out as plaintext.
	disks := []*block.Disk{disk("/dev/vdb")}

	volumes := []*block.DiscoveredVolume{
		volume("vdb", "/dev/vdb", ""),
		luksVolume("vdb1", "/dev/vdb1", "/dev/vdb"),
		volume("vdb2", "/dev/vdb2", "/dev/vdb"),
	}

	got, err := blockhelpers.BuildMatchContexts(disks, volumes, nil, nil, "")
	require.NoError(t, err)

	byPath := contextsByPath(t, got)

	assert.NotContains(t, byPath, "/dev/vdb1")
	assert.Contains(t, byPath, "/dev/vdb2")
}

func TestBuildMatchContextsEncryptedWholeDisk(t *testing.T) {
	// A whole-disk volume on /dev/md0, opened as /dev/dm-0, with the array itself
	// partitioned by nothing. Partitioned is computed on the opened device, and
	// system disk membership is carried across the redirect.
	disks := []*block.Disk{disk("/dev/md0"), disk("/dev/vda"), disk("/dev/dm-0"), disk("/dev/dm-1")}

	volumes := []*block.DiscoveredVolume{
		volume("md0", "/dev/md0", ""),
		volume("dm-0", "/dev/dm-0", ""),
		// the opened device is itself partitioned, so it cannot back a PV directly
		volume("dm-0p1", "/dev/dm-0p1", "/dev/dm-0"),
		// system disk, encrypted: its opened device must stay ineligible
		volume("vda", "/dev/vda", ""),
		volume("dm-1", "/dev/dm-1", ""),
	}

	statuses := []*block.VolumeStatus{
		volumeStatus("u-lvmdata", "/dev/md0", "/dev/dm-0"),
		volumeStatus("STATE", "/dev/vda", "/dev/dm-1"),
	}

	got, err := blockhelpers.BuildMatchContexts(disks, volumes, statuses, nil, "/dev/vda")
	require.NoError(t, err)

	byPath := contextsByPath(t, got)

	require.Contains(t, byPath, "/dev/dm-0")
	assert.Equal(t, "u-lvmdata", byPath["/dev/dm-0"].VolumeID)
	assert.True(t, byPath["/dev/dm-0"].Disk)
	assert.True(t, byPath["/dev/dm-0"].Partitioned)

	require.Contains(t, byPath, "/dev/dm-1")
	assert.True(t, byPath["/dev/dm-1"].SystemDisk)
	assert.Equal(t, "STATE", byPath["/dev/dm-1"].VolumeID)
	// bound from the ciphertext, so `disk` predicates still describe the real disk
	assert.Equal(t, "/dev/vda", byPath["/dev/dm-1"].CELContext["disk"].(*blockpb.DiskSpec).DevPath)
}
