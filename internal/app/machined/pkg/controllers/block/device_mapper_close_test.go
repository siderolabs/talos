// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/go-blockdevice/v2/devicemapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

const (
	dmCloseFinalizer = "block.DeviceMapperCloseController"
	dmMajor          = 252
)

// fakeDeviceMapper keeps device-mapper devices by name, and records the order they are removed in.
type fakeDeviceMapper struct {
	mu sync.Mutex

	devices map[string]uint64
	busy    map[string]struct{}
	removed []string
}

func (f *fakeDeviceMapper) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.devices = map[string]uint64{}
	f.busy = map[string]struct{}{}
	f.removed = nil
}

func (f *fakeDeviceMapper) add(name string, minor uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.devices[name] = unix.Mkdev(dmMajor, minor)
}

func (f *fakeDeviceMapper) setBusy(name string, busy bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if busy {
		f.busy[name] = struct{}{}
	} else {
		delete(f.busy, name)
	}
}

func (f *fakeDeviceMapper) removedDevices() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.removed)
}

func (f *fakeDeviceMapper) ListDevices() ([]devicemapper.DeviceInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	devices := make([]devicemapper.DeviceInfo, 0, len(f.devices))

	for name, devNo := range f.devices {
		devices = append(devices, devicemapper.DeviceInfo{Name: name, DevNo: devNo})
	}

	return devices, nil
}

func (f *fakeDeviceMapper) RemoveDevice(id devicemapper.DeviceID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.busy[id.Name]; ok {
		return fmt.Errorf("failed to remove device-mapper device %s: %w", id, unix.EBUSY)
	}

	if _, ok := f.devices[id.Name]; !ok {
		return fmt.Errorf("failed to remove device-mapper device %s: %w", id, unix.ENXIO)
	}

	delete(f.devices, id.Name)
	f.removed = append(f.removed, id.Name)

	return nil
}

func (f *fakeDeviceMapper) Close() error {
	return nil
}

type DeviceMapperCloseSuite struct {
	ctest.DefaultSuite

	dm *fakeDeviceMapper
}

func TestDeviceMapperCloseSuite(t *testing.T) {
	t.Parallel()

	dm := &fakeDeviceMapper{}

	suite.Run(t, &DeviceMapperCloseSuite{
		dm: dm,
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				dm.reset()

				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.DeviceMapperCloseController{
					NewControl:    func() (blockctrls.DeviceMapperControl, error) { return dm, nil },
					RetryInterval: 50 * time.Millisecond,
				}))
			},
		},
	})
}

func (suite *DeviceMapperCloseSuite) createDevice(id, deviceType, parent string, secondaries ...string) {
	device := block.NewDevice(block.NamespaceName, id)
	device.TypedSpec().Type = deviceType
	device.TypedSpec().Parent = parent
	device.TypedSpec().Secondaries = secondaries

	var minor int

	if _, err := fmt.Sscanf(id, "dm-%d", &minor); err == nil {
		device.TypedSpec().Major = dmMajor
		device.TypedSpec().Minor = minor

		suite.dm.add("dm-name-"+id, uint32(minor))
	}

	suite.Create(device)
}

func (suite *DeviceMapperCloseSuite) createVolumeStatus(id string, volumeType block.VolumeType, location, mountLocation string) *block.VolumeStatus {
	volumeStatus := block.NewVolumeStatus(block.NamespaceName, id)
	volumeStatus.TypedSpec().Type = volumeType
	volumeStatus.TypedSpec().Phase = block.VolumePhaseReady
	volumeStatus.TypedSpec().Location = location
	volumeStatus.TypedSpec().MountLocation = mountLocation

	suite.Create(volumeStatus)

	return volumeStatus
}

func (suite *DeviceMapperCloseSuite) teardown(volumeStatus *block.VolumeStatus) {
	_, err := suite.State().Teardown(suite.Ctx(), volumeStatus.Metadata())
	suite.Require().NoError(err)
}

func (suite *DeviceMapperCloseSuite) assertFinalizer(id string, present bool) {
	ctest.AssertResource(suite, id, func(volumeStatus *block.VolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(present, volumeStatus.Metadata().Finalizers().Has(dmCloseFinalizer))
	})
}

// createLVMOnLUKS sets up an encrypted raw volume (vda4 -> dm-0), with an LVM logical volume (dm-1) on top,
// which in turn is partitioned (dm-2), and a raid1 logical volume (dm-5) with its private sub-LVs (dm-3, dm-4).
func (suite *DeviceMapperCloseSuite) createLVMOnLUKS() *block.VolumeStatus {
	suite.createDevice("vda", block.DeviceTypeDisk, "")
	suite.createDevice("vda4", block.DeviceTypePartition, "vda")
	suite.createDevice("dm-0", block.DeviceTypeDisk, "", "vda4")
	suite.createDevice("dm-1", block.DeviceTypeDisk, "", "dm-0")
	suite.createDevice("dm-2", block.DeviceTypePartition, "dm-1")
	suite.createDevice("dm-3", block.DeviceTypeDisk, "", "dm-0")
	suite.createDevice("dm-4", block.DeviceTypeDisk, "", "dm-0")
	suite.createDevice("dm-5", block.DeviceTypeDisk, "", "dm-3", "dm-4")

	return suite.createVolumeStatus("r-lvm", block.VolumeTypePartition, "/dev/vda4", "/dev/dm-0")
}

func (suite *DeviceMapperCloseSuite) TestFinalizers() {
	suite.createVolumeStatus("EPHEMERAL", block.VolumeTypePartition, "/dev/vda5", "/dev/vda5")
	suite.createVolumeStatus("/var/lib", block.VolumeTypeDirectory, "", "")

	suite.assertFinalizer("EPHEMERAL", true)

	time.Sleep(100 * time.Millisecond)

	suite.assertFinalizer("/var/lib", false)
}

func (suite *DeviceMapperCloseSuite) TestRemovesStackedDevices() {
	rawVolume := suite.createLVMOnLUKS()
	suite.assertFinalizer("r-lvm", true)

	suite.teardown(rawVolume)
	suite.assertFinalizer("r-lvm", false)

	removed := suite.dm.removedDevices()
	suite.Assert().ElementsMatch([]string{"dm-name-dm-1", "dm-name-dm-2", "dm-name-dm-3", "dm-name-dm-4", "dm-name-dm-5"}, removed)

	// topmost first, and the opened device of the volume itself is left to the volume manager
	suite.Assert().Less(slices.Index(removed, "dm-name-dm-2"), slices.Index(removed, "dm-name-dm-1"))
	suite.Assert().Less(slices.Index(removed, "dm-name-dm-5"), slices.Index(removed, "dm-name-dm-3"))
	suite.Assert().Less(slices.Index(removed, "dm-name-dm-5"), slices.Index(removed, "dm-name-dm-4"))
}

func (suite *DeviceMapperCloseSuite) TestWaitsForStackedVolume() {
	rawVolume := suite.createLVMOnLUKS()
	userVolume := suite.createVolumeStatus("u-data", block.VolumeTypePartition, "/dev/dm-2", "/dev/dm-2")

	suite.assertFinalizer("r-lvm", true)
	suite.teardown(rawVolume)

	time.Sleep(200 * time.Millisecond)

	suite.assertFinalizer("r-lvm", true)
	suite.Assert().NotContains(suite.dm.removedDevices(), "dm-name-dm-2")

	// the volume stacked on top is closed by the volume manager
	suite.teardown(userVolume)
	suite.assertFinalizer("u-data", false)
	suite.Destroy(userVolume)

	suite.assertFinalizer("r-lvm", false)
	suite.Assert().Contains(suite.dm.removedDevices(), "dm-name-dm-2")
}

func (suite *DeviceMapperCloseSuite) TestRetriesBusyDevice() {
	rawVolume := suite.createLVMOnLUKS()
	suite.dm.setBusy("dm-name-dm-1", true)

	suite.assertFinalizer("r-lvm", true)
	suite.teardown(rawVolume)

	time.Sleep(200 * time.Millisecond)

	suite.assertFinalizer("r-lvm", true)

	suite.dm.setBusy("dm-name-dm-1", false)

	suite.assertFinalizer("r-lvm", false)
	suite.Assert().Contains(suite.dm.removedDevices(), "dm-name-dm-1")
}

func (suite *DeviceMapperCloseSuite) TestWaitsForNonDeviceMapperHolder() {
	suite.createDevice("vda", block.DeviceTypeDisk, "")
	suite.createDevice("vda4", block.DeviceTypePartition, "vda")
	suite.createDevice("md127", block.DeviceTypeDisk, "", "vda4")

	volume := suite.createVolumeStatus("r-md", block.VolumeTypePartition, "/dev/vda4", "/dev/vda4")

	suite.assertFinalizer("r-md", true)
	suite.teardown(volume)

	time.Sleep(200 * time.Millisecond)

	suite.assertFinalizer("r-md", true)
	suite.Assert().Empty(suite.dm.removedDevices())
}
