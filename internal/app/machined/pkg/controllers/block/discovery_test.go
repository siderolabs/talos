// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/google/uuid"
	blkdev "github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type DiscoverySuite struct {
	ctest.DefaultSuite
}

func TestDiscoverySuite(t *testing.T) {
	suite.Run(t, new(DiscoverySuite))
}

// TestPartitionProbedAlone checks that a partition which is probed on its own, without its disk,
// keeps the offset learned from the partition table of the disk.
//
// This is what happens when a partition gets formatted: the change event is for the partition
// only, so only the partition is probed again.
func (suite *DiscoverySuite) TestPartitionProbedAlone() {
	if os.Geteuid() != 0 {
		suite.T().Skip("skipping test; must be root to use loop devices")
	}

	if hostname, _ := os.Hostname(); hostname == "buildkitsandbox" { //nolint:errcheck
		suite.T().Skip("skipping test; partition devices are not propagated to /dev under buildkit")
	}

	if _, err := exec.LookPath("mkfs.vfat"); err != nil {
		suite.T().Skip("skipping test; mkfs.vfat is not available")
	}

	loDev := attachLoopDevice(suite.T(), createRawImage(suite.T(), 16*1024*1024))
	diskID := filepath.Base(loDev.Path())

	// the partition under test is the second one, so that its offset is not the first usable sector
	prepareGPT(suite.T(), loDev.Path(), 2)

	partIDs := []string{diskID + "p1", diskID + "p2"}
	partID := partIDs[1]

	// partition devices are created by the kernel when the partition table is written
	suite.Require().EventuallyWithT(func(c *assert.CollectT) {
		_, err := os.Stat(filepath.Join("/dev", partID))
		assert.NoError(c, err)
	}, 10*time.Second, 50*time.Millisecond)

	expectedOffset := partitionOffset(suite.T(), partID)
	suite.Require().NotZero(expectedOffset)

	suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.DiscoveryController{}))

	suite.Create(newDiskDevice(diskID, 1))

	suite.Create(newPartitionDevice(partIDs[0], diskID, 1))

	partResource := newPartitionDevice(partIDs[1], diskID, 2)

	suite.Create(partResource)

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), partID, func(dv *block.DiscoveredVolume, asrt *assert.Assertions) {
		asrt.Equal(expectedOffset, dv.TypedSpec().Offset)
		asrt.Empty(dv.TypedSpec().Name)
	})

	// format the partition, and deliver the change event for the partition only
	cmd := exec.CommandContext(suite.Ctx(), "mkfs.vfat", filepath.Join("/dev", partID))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	suite.Require().NoError(cmd.Run())

	partResource.TypedSpec().Generation++
	suite.Update(partResource)

	// the filesystem shows that the partition was probed again, while the offset stays
	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), partID, func(dv *block.DiscoveredVolume, asrt *assert.Assertions) {
		asrt.Equal("vfat", dv.TypedSpec().Name)
		asrt.Equal(expectedOffset, dv.TypedSpec().Offset)
	})
}

func newPartitionDevice(id, parent string, generation int) *block.Device {
	dev := block.NewDevice(block.NamespaceName, id)
	dev.TypedSpec().Type = block.DeviceTypePartition
	dev.TypedSpec().DevicePath = filepath.Join("/sys/class/block", id)
	dev.TypedSpec().Parent = parent
	dev.TypedSpec().Generation = generation

	return dev
}

// prepareGPT writes a GPT partition table with the specified number of 1 MiB partitions.
func prepareGPT(t *testing.T, path string, partitions int) {
	t.Helper()

	dev, err := blkdev.NewFromPath(path, blkdev.OpenForWrite())
	require.NoError(t, err)

	defer dev.Close() //nolint:errcheck

	gptdev, err := gpt.DeviceFromBlockDevice(dev)
	require.NoError(t, err)

	pt, err := gpt.New(gptdev)
	require.NoError(t, err)

	for i := range partitions {
		_, _, err = pt.AllocatePartition(1024*1024, fmt.Sprintf("PART%d", i+1), uuid.MustParse("0fc63daf-8483-4772-8e79-3d69d8477de4"))
		require.NoError(t, err)
	}

	require.NoError(t, pt.Write())
}

// partitionOffset returns the offset of the partition in bytes, as reported by the kernel.
func partitionOffset(t *testing.T, id string) uint64 {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join("/sys/class/block", id, "start"))
	require.NoError(t, err)

	// sysfs reports the start in 512-byte sectors regardless of the sector size of the device
	start, err := strconv.ParseUint(strings.TrimSpace(string(contents)), 10, 64)
	require.NoError(t, err)

	return start * 512
}
