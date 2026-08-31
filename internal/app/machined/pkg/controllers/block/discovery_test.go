// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
)

// TestGPTNestedProbeResults verifies recovering GPT partitions on a "hybrid" disk, where a whole-disk
// filesystem signature (e.g. ISO9660 on a Talos ISO written to a USB stick) shadows the partition table
// from the blkid probe, so the partitions have to be discovered by reading the GPT directly.
func TestGPTNestedProbeResults(t *testing.T) {
	t.Parallel()

	const diskSize = 4 * 1024 * 1024

	partType := uuid.MustParse("0FC63DAF-8483-4772-8E79-3D69D8477DE4") // Linux filesystem data

	imagePath := filepath.Join(t.TempDir(), "image.raw")

	f, err := os.OpenFile(imagePath, os.O_CREATE|os.O_RDWR, 0o644)
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, f.Close()) })

	require.NoError(t, f.Truncate(diskSize))

	gptdev, err := gpt.DeviceFromFile(f)
	require.NoError(t, err)

	// without a GPT on the disk, expect an error
	_, err = blockctrls.GPTNestedProbeResults(gptdev, map[uint]string{1: "sda1"})
	require.Error(t, err)

	pt, err := gpt.New(gptdev)
	require.NoError(t, err)

	_, part1, err := pt.AllocatePartition(1024*1024, "metal-iso", partType)
	require.NoError(t, err)

	_, part2, err := pt.AllocatePartition(1024*1024, "", partType)
	require.NoError(t, err)

	require.NoError(t, pt.Write())

	sectorSize := uint64(gptdev.GetSectorSize())

	// both partitions published by the kernel
	parts, err := blockctrls.GPTNestedProbeResults(gptdev, map[uint]string{1: "sda1", 2: "sda2"})
	require.NoError(t, err)
	require.Len(t, parts, 2)

	assert.Equal(t, uint(1), parts[0].PartitionIndex)
	require.NotNil(t, parts[0].PartitionLabel)
	assert.Equal(t, "metal-iso", *parts[0].PartitionLabel)
	require.NotNil(t, parts[0].PartitionUUID)
	assert.Equal(t, part1.PartGUID, *parts[0].PartitionUUID)
	require.NotNil(t, parts[0].PartitionType)
	assert.Equal(t, partType, *parts[0].PartitionType)
	assert.Equal(t, part1.FirstLBA*sectorSize, parts[0].PartitionOffset)
	assert.Equal(t, (part1.LastLBA-part1.FirstLBA+1)*sectorSize, parts[0].PartitionSize)

	assert.Equal(t, uint(2), parts[1].PartitionIndex)
	require.NotNil(t, parts[1].PartitionLabel)
	assert.Equal(t, "", *parts[1].PartitionLabel)
	assert.Equal(t, part2.FirstLBA*sectorSize, parts[1].PartitionOffset)

	// partitions not published by the kernel are skipped
	parts, err = blockctrls.GPTNestedProbeResults(gptdev, map[uint]string{2: "sda2"})
	require.NoError(t, err)
	require.Len(t, parts, 1)

	assert.Equal(t, uint(2), parts[0].PartitionIndex)

	// no partitions published by the kernel
	parts, err = blockctrls.GPTNestedProbeResults(gptdev, nil)
	require.NoError(t, err)
	assert.Empty(t, parts)
}
