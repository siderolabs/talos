// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bootloader_test

import (
	"errors"
	randv2 "math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/freddierice/go-losetup/v2"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

func checkRequirements(t *testing.T) {
	t.Helper()

	if os.Geteuid() != 0 {
		t.Skip("test requires root privileges")
	}

	if hostname, _ := os.Hostname(); hostname == "buildkitsandbox" { //nolint: errcheck
		t.Skip("test not supported under buildkit as partition devices are not propagated from /dev")
	}
}

func losetupAttachHelper(t *testing.T, rawImage string, readonly bool) losetup.Device {
	t.Helper()

	for range 10 {
		loDev, err := losetup.Attach(rawImage, 0, readonly)
		if err != nil {
			if errors.Is(err, unix.EBUSY) {
				spraySleep := max(randv2.ExpFloat64(), 2.0)

				t.Logf("retrying after %v seconds", spraySleep)

				time.Sleep(time.Duration(spraySleep * float64(time.Second)))

				continue
			}
		}

		require.NoError(t, err)

		return loDev
	}

	t.Fatal("failed to attach loop device") //nolint:revive

	panic("unreachable")
}

func prepareRawImage(t *testing.T, size int64) string {
	t.Helper()

	tmpDir := t.TempDir()

	rawImage := filepath.Join(tmpDir, "image.raw")

	f, err := os.Create(rawImage)
	require.NoError(t, err)

	require.NoError(t, f.Truncate(size))
	require.NoError(t, f.Close())

	loDev := losetupAttachHelper(t, rawImage, false)

	t.Cleanup(func() {
		assert.NoError(t, loDev.Detach())
	})

	return loDev.Path()
}

const mib = 1024 * 1024

func TestCleanup(t *testing.T) {
	checkRequirements(t)

	disk := prepareRawImage(t, 2*1024*mib)

	dev, err := block.NewFromPath(disk, block.OpenForWrite())
	assert.NoError(t, err)

	cleanupFunc := sync.OnceValue(dev.Close)

	t.Cleanup(func() {
		assert.NoError(t, cleanupFunc())
	})

	gptDev, err := gpt.DeviceFromBlockDevice(dev)
	assert.NoError(t, err)

	pt, err := gpt.New(gptDev, gpt.WithMarkPMBRBootable())
	assert.NoError(t, err)

	quirk := quirks.New("")

	partitions := []partition.Options{ //nolint:prealloc // this is a test
		partition.NewPartitionOptions(false, quirk, partition.WithLabel(constants.EFIPartitionLabel)),
		partition.NewPartitionOptions(false, quirk, partition.WithLabel(constants.BIOSGrubPartitionLabel)),
		partition.NewPartitionOptions(false, quirk, partition.WithLabel(constants.BootPartitionLabel)),
	}

	partitions = append(partitions, partition.NewPartitionOptions(false, quirks.New(""), partition.WithLabel(constants.MetaPartitionLabel)))

	for _, p := range partitions {
		size := p.Size

		if size == 0 {
			size = pt.LargestContiguousAllocatable()
		}

		partitionTyp := uuid.MustParse(p.PartitionType)

		_, _, err = pt.AllocatePartition(size, p.PartitionLabel, partitionTyp, p.PartitionOpts...)
		assert.NoError(t, err)
	}

	assert.NoError(t, pt.Write())

	// close operations on the disk
	assert.NoError(t, cleanupFunc())

	assert.NoError(t, bootloader.CleanupBootloader(disk, false))

	testPartitionsWiped(t, disk, []string{constants.BIOSGrubPartitionLabel, constants.BootPartitionLabel, constants.MetaPartitionLabel}, false)

	assert.NoError(t, bootloader.CleanupBootloader(disk, true))

	testPartitionsWiped(t, disk, []string{constants.MetaPartitionLabel}, true)
}

func testPartitionsWiped(t *testing.T, disk string, expectedLabels []string, sdboot bool) {
	dev, err := block.NewFromPath(disk)
	assert.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, dev.Close())
	})

	gptDev, err := gpt.DeviceFromBlockDevice(dev)
	assert.NoError(t, err)

	pt, err := gpt.Read(gptDev)
	assert.NoError(t, err)

	labels := xslices.Filter(xslices.Map(pt.Partitions(), func(p *gpt.Partition) string {
		if p == nil {
			return ""
		}

		return p.Name
	}), func(label string) bool {
		return label != ""
	})

	assert.Equal(t, expectedLabels, labels)

	if sdboot {
		var mbrData [446]byte

		_, err = dev.File().Read(mbrData[:])
		assert.NoError(t, err)

		assert.Equal(t, [446]byte{}, mbrData)
	}
}
