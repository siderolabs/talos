// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"errors"
	"fmt"
	randv2 "math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	blkdev "github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-blockdevice/v2/devicemapper"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type MultipathSuite struct {
	ctest.DefaultSuite
}

func TestMultipathSuite(t *testing.T) {
	suite.Run(t, new(MultipathSuite))
}

const multipathImageSize = 16 * 1024 * 1024

// TestPathsHidden checks that once a multipath device is assembled over its paths, the paths are
// hidden: the volumes of the disk are only discovered via the multipath device, and the paths are
// not disks of their own anymore.
//
// The paths are two loop devices backed by the same image, so that, just like the paths of a real
// multipath device, all three devices read the same partition table.
func (suite *MultipathSuite) TestPathsHidden() {
	if os.Geteuid() != 0 {
		suite.T().Skip("skipping test; must be root to use loop devices")
	}

	if _, err := os.Stat(devicemapper.ControlPath); err != nil {
		suite.T().Skip("skipping test; device-mapper control device is not available")
	}

	image := createRawImage(suite.T(), multipathImageSize)

	pathA := attachLoopDevice(suite.T(), image)
	writeMETAPartition(suite.T(), pathA.Path())

	// the kernel partitions of a path are created as the table is written, so write it once again via
	// the other path, just like the kernel scans the partition table of each path of a real disk
	pathB := attachLoopDevice(suite.T(), image)
	writeMETAPartition(suite.T(), pathB.Path())

	pathAID := filepath.Base(pathA.Path())
	pathBID := filepath.Base(pathB.Path())

	suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.DiscoveryController{}))
	suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.DisksController{}))

	// before the multipath device is assembled, every path is a disk, and META is discovered on each of them
	suite.Create(newDiskDevice(pathAID, 1))
	suite.Create(newDiskDevice(pathBID, 1))

	ctest.AssertResources(suite, []string{pathAID, pathBID}, func(*block.Disk, *assert.Assertions) {})
	suite.assertMETAParents(pathAID, pathBID)

	// assemble the multipath device over both paths
	mpath := createMultipathDevice(suite.T(), pathA.Path(), pathB.Path())

	mpathDevice := newDiskDevice(mpath, 1)
	mpathDevice.TypedSpec().Secondaries = []string{pathAID, pathBID}
	suite.Create(mpathDevice)

	// now META is only discovered via the multipath device, and the paths are no longer disks
	suite.assertMETAParents(mpath)
	suite.assertNoVolumesOf(pathAID, pathBID)

	ctest.AssertNoResource[*block.Disk](suite, pathAID)
	ctest.AssertNoResource[*block.Disk](suite, pathBID)
	ctest.AssertResource(suite, mpath, func(disk *block.Disk, asrt *assert.Assertions) {
		asrt.Equal("mpath", disk.TypedSpec().DeviceMapperKind)
		asrt.ElementsMatch([]string{pathAID, pathBID}, disk.TypedSpec().SecondaryDisks)
		asrt.EqualValues(multipathImageSize, disk.TypedSpec().Size)
	})

	// tear down the multipath device: the paths are disks once again
	removeMultipathDevice(suite.T(), mpath)
	suite.Destroy(mpathDevice)

	ctest.AssertResources(suite, []string{pathAID, pathBID}, func(*block.Disk, *assert.Assertions) {})
	ctest.AssertNoResource[*block.Disk](suite, mpath)
	suite.assertMETAParents(pathAID, pathBID)
}

// assertMETAParents asserts that the META partition is discovered on exactly the given disks.
func (suite *MultipathSuite) assertMETAParents(parents ...string) {
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		volumes, err := safe.StateListAll[*block.DiscoveredVolume](suite.Ctx(), suite.State())
		require.NoError(collect, err)

		var actual []string

		for dv := range volumes.All() {
			if dv.TypedSpec().PartitionLabel == "META" {
				actual = append(actual, dv.TypedSpec().Parent)
			}
		}

		assert.ElementsMatch(collect, parents, actual)
	}, 10*time.Second, 50*time.Millisecond)
}

// assertNoVolumesOf asserts that nothing is discovered on the given disks, neither the disks
// themselves nor their partitions.
func (suite *MultipathSuite) assertNoVolumesOf(ids ...string) {
	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		volumes, err := safe.StateListAll[*block.DiscoveredVolume](suite.Ctx(), suite.State())
		require.NoError(collect, err)

		for dv := range volumes.All() {
			assert.NotContains(collect, ids, dv.Metadata().ID())
			assert.NotContains(collect, ids, dv.TypedSpec().Parent)
		}
	}, 10*time.Second, 50*time.Millisecond)
}

// writeMETAPartition writes a GPT with a single META partition to the block device.
func writeMETAPartition(t *testing.T, path string) {
	t.Helper()

	dev, err := blkdev.NewFromPath(path, blkdev.OpenForWrite())
	require.NoError(t, err)

	defer dev.Close() //nolint:errcheck

	gptdev, err := gpt.DeviceFromBlockDevice(dev)
	require.NoError(t, err)

	pt, err := gpt.New(gptdev)
	require.NoError(t, err)

	_, _, err = pt.AllocatePartition(1024*1024, "META", uuid.MustParse("0fc63daf-8483-4772-8e79-3d69d8477de4")) // Linux filesystem data
	require.NoError(t, err)

	require.NoError(t, pt.Write())
}

// createMultipathDevice creates a device-mapper device which is a multipath device over the given
// paths, and returns its kernel name ("dm-N").
//
// The real multipath target is used if the kernel has it. Otherwise, the device is mapped half over
// one path and half over the other, which is the very same thing as far as Talos can tell: a
// device-mapper device carrying a multipath UUID, holding both paths, and reading the same data as
// each of them.
func createMultipathDevice(t *testing.T, paths ...string) string {
	t.Helper()

	control, err := devicemapper.NewControl()
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, control.Close())
	})

	const sectors = multipathImageSize / 512

	devNos := xslices.Map(paths, func(path string) string {
		var st unix.Stat_t

		require.NoError(t, unix.Stat(path, &st))

		return fmt.Sprintf("%d:%d", unix.Major(st.Rdev), unix.Minor(st.Rdev))
	})

	suffix := fmt.Sprintf("%08x", randv2.Uint32())
	id := devicemapper.DeviceID{
		Name: "talos-mpath-" + suffix,
		UUID: "mpath-3600508" + suffix,
	}

	err = control.CreateDevice(id, []devicemapper.Target{
		{
			Type:   "multipath",
			Params: fmt.Sprintf("0 0 1 1 round-robin 0 %d 1 %s 1 %s 1", len(devNos), devNos[0], devNos[1]),
			Length: sectors,
		},
	})
	if errors.Is(err, unix.EINVAL) {
		t.Log("kernel has no multipath target, mapping the paths linearly")

		err = control.CreateDevice(id, []devicemapper.Target{
			{
				Type:   "linear",
				Params: devNos[0] + " 0",
				Length: sectors / 2,
			},
			{
				Type:        "linear",
				Params:      fmt.Sprintf("%s %d", devNos[1], sectors/2),
				SectorStart: sectors / 2,
				Length:      sectors / 2,
			},
		})
	}

	require.NoError(t, err)

	devName := ""

	devices, err := control.ListDevices()
	require.NoError(t, err)

	for _, device := range devices {
		if device.Name != id.Name {
			continue
		}

		devName = fmt.Sprintf("dm-%d", unix.Minor(device.DevNo))

		// there is no udev to create the device node in a test environment
		devPath := filepath.Join("/dev", devName)

		if _, err = os.Stat(devPath); errors.Is(err, os.ErrNotExist) {
			require.NoError(t, unix.Mknod(devPath, unix.S_IFBLK|0o600, int(device.DevNo)))

			t.Cleanup(func() {
				assert.NoError(t, os.Remove(devPath))
			})
		}
	}

	require.NotEmpty(t, devName)

	t.Cleanup(func() {
		removeMultipathDevice(t, devName)
	})

	return devName
}

// removeMultipathDevice removes the multipath device along with the partition maps Talos laid over it.
//
// It does nothing if the device is gone already.
func removeMultipathDevice(t *testing.T, devName string) {
	t.Helper()

	uuid, err := os.ReadFile(filepath.Join("/sys/class/block", devName, "dm", "uuid"))
	if errors.Is(err, os.ErrNotExist) {
		return
	}

	require.NoError(t, err)

	control, err := devicemapper.NewControl()
	require.NoError(t, err)

	defer control.Close() //nolint:errcheck

	holders, err := os.ReadDir(filepath.Join("/sys/class/block", devName, "holders"))
	require.NoError(t, err)

	// the devices might still be held open by a probe for a moment, so removal is retried
	remove := func(uuid []byte) {
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			assert.NoError(collect, control.RemoveDevice(devicemapper.ByUUID(strings.TrimSpace(string(uuid)))))
		}, 10*time.Second, 100*time.Millisecond)
	}

	for _, holder := range holders {
		holderUUID, err := os.ReadFile(filepath.Join("/sys/class/block", holder.Name(), "dm", "uuid"))
		require.NoError(t, err)

		remove(holderUUID)
	}

	remove(uuid)

	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join("/sys/class/block", devName))

		return errors.Is(err, os.ErrNotExist)
	}, 10*time.Second, 50*time.Millisecond)
}

func TestNativeMultipathEnabled(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		cmdline  string
		expected bool
	}{
		{cmdline: "console=ttyS0", expected: false},
		{cmdline: "talos.multipath=on", expected: true},
		{cmdline: "talos.multipath=ON", expected: true},
		{cmdline: "talos.multipath=1", expected: true},
		{cmdline: "talos.multipath=true", expected: true},
		{cmdline: "talos.multipath=off", expected: false},
		{cmdline: "talos.multipath=0", expected: false},
		{cmdline: "talos.multipath=bogus", expected: false},
	} {
		t.Run(test.cmdline, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, blockctrls.NativeMultipathEnabled(procfs.NewCmdline(test.cmdline)))
		})
	}

	assert.False(t, blockctrls.NativeMultipathEnabled(nil))
}
