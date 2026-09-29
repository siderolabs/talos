// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/freddierice/go-losetup/v2"
	blkdev "github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

type DisksSuite struct {
	ctest.DefaultSuite
}

func TestDisksSuite(t *testing.T) {
	suite.Run(t, new(DisksSuite))
}

const loopImageSize = 4 * 1024 * 1024

// TestRecreatedDevice checks that a device which goes away and comes back under the same name
// is probed again, even if the first incarnation never produced a disk.
//
// This is what happens with device-mapper devices: "dm-N" names are reused, and a private
// (or not yet resumed) device-mapper device is skipped by the controller.
func (suite *DisksSuite) TestRecreatedDevice() {
	if os.Geteuid() != 0 {
		suite.T().Skip("skipping test; must be root to use loop devices")
	}

	// the subject is a loop device which is going to be detached and attached back
	subjectImage := createRawImage(suite.T(), loopImageSize)
	subject := attachLoopDevice(suite.T(), subjectImage)

	// the barrier is a loop device which stays attached; once the controller has produced a disk for it,
	// it has processed everything created before it
	barrierImage := createRawImage(suite.T(), loopImageSize)
	barrier := attachLoopDevice(suite.T(), barrierImage)

	subjectID := filepath.Base(subject.Path())
	barrierID := filepath.Base(barrier.Path())

	suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.DisksController{}))

	// detach the subject: the device node stays, but the device has no size now
	suite.Require().NoError(subject.Detach())
	waitForLoopDeviceSize(suite.T(), subject.Path(), 0)

	// first incarnation: the controller sees an empty device and produces no disk for it
	const generation = 2

	suite.Create(newDiskDevice(subjectID, generation))
	suite.Create(newDiskDevice(barrierID, 1))

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), barrierID, func(disk *block.Disk, asrt *assert.Assertions) {
		asrt.EqualValues(loopImageSize, disk.TypedSpec().Size)
	})
	ctest.AssertNoResource[*block.Disk](suite, subjectID)

	// the device goes away
	suite.Destroy(newDiskDevice(subjectID, generation))
	ctest.AssertNoResource[*block.Disk](suite, subjectID)

	// second incarnation of the device under the same name and with the same generation number, but now it has a size
	reattachLoopDevice(suite.T(), subject, subjectImage)
	waitForLoopDeviceSize(suite.T(), subject.Path(), loopImageSize)

	suite.Create(newDiskDevice(subjectID, generation))

	rtestutils.AssertResource(suite.Ctx(), suite.T(), suite.State(), subjectID, func(disk *block.Disk, asrt *assert.Assertions) {
		asrt.EqualValues(loopImageSize, disk.TypedSpec().Size)
	})
}

func newDiskDevice(id string, generation int) *block.Device {
	dev := block.NewDevice(block.NamespaceName, id)
	dev.TypedSpec().Type = block.DeviceTypeDisk
	dev.TypedSpec().DevicePath = filepath.Join("/sys/class/block", id)
	dev.TypedSpec().Generation = generation

	return dev
}

func createRawImage(t *testing.T, size int64) string {
	t.Helper()

	rawImage := filepath.Join(t.TempDir(), "image.raw")

	f, err := os.Create(rawImage)
	require.NoError(t, err)

	require.NoError(t, f.Truncate(size))
	require.NoError(t, f.Close())

	return rawImage
}

func attachLoopDevice(t *testing.T, rawImage string) losetup.Device {
	t.Helper()

	var (
		loDev losetup.Device
		err   error
	)

	for range 10 {
		loDev, err = losetup.Attach(rawImage, 0, false)
		if errors.Is(err, unix.EBUSY) {
			time.Sleep(100 * time.Millisecond)

			continue
		}

		break
	}

	require.NoError(t, err)

	t.Cleanup(func() {
		err := loDev.Detach()
		if err != nil && !errors.Is(err, unix.ENXIO) {
			assert.NoError(t, err)
		}
	})

	return loDev
}

// reattachLoopDevice attaches the backing file to the exact loop device once again.
func reattachLoopDevice(t *testing.T, loDev losetup.Device, rawImage string) {
	t.Helper()

	back, err := os.OpenFile(rawImage, os.O_RDWR, 0)
	require.NoError(t, err)

	defer back.Close() //nolint:errcheck

	loopFile, err := os.OpenFile(loDev.Path(), os.O_RDWR, 0)
	require.NoError(t, err)

	defer loopFile.Close() //nolint:errcheck

	err = unix.IoctlSetInt(int(loopFile.Fd()), unix.LOOP_SET_FD, int(back.Fd()))
	if errors.Is(err, unix.EBUSY) {
		t.Skipf("loop device %s was taken by another process", loDev.Path())
	}

	require.NoError(t, err)
}

func waitForLoopDeviceSize(t *testing.T, path string, expected uint64) {
	t.Helper()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		dev, err := blkdev.NewFromPath(path)
		require.NoError(c, err)

		defer dev.Close() //nolint:errcheck

		size, err := dev.GetSize()
		require.NoError(c, err)

		assert.Equal(c, expected, size)
	}, 10*time.Second, 50*time.Millisecond)
}
