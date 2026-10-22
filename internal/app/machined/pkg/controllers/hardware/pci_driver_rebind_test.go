// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hardwarectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	rebindDevice      = "0000:01:00.0"
	rebindOtherDevice = "0000:02:00.0"
)

type PCIDriverRebindSuite struct {
	ctest.DefaultSuite

	sysfs, procfs string
	reconcileCh   chan struct{}
}

func TestPCIDriverRebindSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &PCIDriverRebindSuite{
		Timeout: 10 * time.Second,
		AfterSetup: func(s *ctest.DefaultSuite) {
			devices := runtime.NewDevicesStatus(runtime.NamespaceName, runtime.DevicesID)
			devices.TypedSpec().Ready = true
			s.Require().NoError(s.State().Create(s.Ctx(), devices))
		},
	})
}

func (suite *PCIDriverRebindSuite) SetupTest() {
	suite.DefaultSuite.SetupTest()

	suite.sysfs = filepath.Join(suite.T().TempDir(), "sys")
	suite.procfs = filepath.Join(suite.T().TempDir(), "proc")

	suite.Require().NoError(os.MkdirAll(filepath.Join(suite.sysfs, "bus", "pci", "devices"), 0o755))
	suite.Require().NoError(os.MkdirAll(suite.procfs, 0o755))
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.sysfs, "bus", "pci", "drivers_probe"), nil, 0o644))

	suite.reconcileCh = make(chan struct{}, 1)
	suite.Require().NoError(suite.Runtime().RegisterController(&hardwarectrl.PCIDriverRebindController{
		SysfsPath:   suite.sysfs,
		ProcfsPath:  suite.procfs,
		ReconcileCh: suite.reconcileCh,
	}))
}

// device lays out a fake PCI device in an IOMMU group, bound to the given driver.
func (suite *PCIDriverRebindSuite) device(id, driver, group string) {
	path := suite.devicePath(id)
	suite.Require().NoError(os.MkdirAll(path, 0o755))
	suite.Require().NoError(os.WriteFile(filepath.Join(path, "driver_override"), []byte("(null)\n"), 0o644))

	groupPath := filepath.Join(suite.sysfs, "kernel", "iommu_groups", group)
	suite.Require().NoError(os.MkdirAll(groupPath, 0o755))
	suite.Require().NoError(os.Symlink(groupPath, filepath.Join(path, "iommu_group")))

	suite.bind(id, driver)
}

// bind points a fake device's driver link at the given driver, as the kernel does on a probe.
func (suite *PCIDriverRebindSuite) bind(id, driver string) {
	drv := filepath.Join(suite.sysfs, "bus", "pci", "drivers", driver)
	suite.Require().NoError(os.MkdirAll(drv, 0o755))

	if _, err := os.Stat(filepath.Join(drv, "unbind")); os.IsNotExist(err) {
		suite.Require().NoError(os.WriteFile(filepath.Join(drv, "unbind"), nil, 0o644))
	}

	link := filepath.Join(suite.devicePath(id), "driver")
	suite.Require().NoError(os.RemoveAll(link))
	suite.Require().NoError(os.Symlink(drv, link))
}

// openVFIOGroup makes a fake process, pid 4242, hold the node of VFIO group 7 open.
func (suite *PCIDriverRebindSuite) openVFIOGroup() {
	fd := filepath.Join(suite.procfs, "4242", "fd")
	suite.Require().NoError(os.MkdirAll(fd, 0o755))
	suite.Require().NoError(os.Symlink("/dev/vfio/7", filepath.Join(fd, "42")))
}

// openVFIODevice makes a fake process hold a VFIO device file descriptor, as obtained from its
// group, whose fdinfo names the given device; an empty one leaves the name out, as older kernels do.
func (suite *PCIDriverRebindSuite) openVFIODevice(pid, id string) {
	fd := filepath.Join(suite.procfs, pid, "fd")
	fdinfo := filepath.Join(suite.procfs, pid, "fdinfo")
	suite.Require().NoError(os.MkdirAll(fd, 0o755))
	suite.Require().NoError(os.MkdirAll(fdinfo, 0o755))
	suite.Require().NoError(os.Symlink("anon_inode:[vfio-device]", filepath.Join(fd, "43")))

	info := "pos:\t0\nflags:\t02000002\nmnt_id:\t17\nino:\t25\n"
	if id != "" {
		info += "vfio-device-syspath: /sys/bus/pci/devices/" + id + "\n"
	}

	suite.Require().NoError(os.WriteFile(filepath.Join(fdinfo, "43"), []byte(info), 0o644))
}

func (suite *PCIDriverRebindSuite) devicePath(id string) string {
	return filepath.Join(suite.sysfs, "bus", "pci", "devices", id)
}

func (suite *PCIDriverRebindSuite) read(path ...string) string {
	contents, err := os.ReadFile(filepath.Join(append([]string{suite.sysfs}, path...)...))
	suite.Require().NoError(err)

	return string(contents)
}

func (suite *PCIDriverRebindSuite) unbound(driver string) string {
	return suite.read("bus", "pci", "drivers", driver, "unbind")
}

func (suite *PCIDriverRebindSuite) override(id string) string {
	return suite.read("bus", "pci", "devices", id, "driver_override")
}

// configure asks for a device to be bound to vfio-pci.
func (suite *PCIDriverRebindSuite) configure(id string) {
	cfg := hardware.NewPCIDriverRebindConfig(id)
	cfg.TypedSpec().PCIID = id
	cfg.TypedSpec().TargetDriver = "vfio-pci"
	suite.Create(cfg)
}

func (suite *PCIDriverRebindSuite) reconcile() {
	suite.reconcileCh <- struct{}{}
}

func (suite *PCIDriverRebindSuite) assertStatus(expected hardware.PCIDriverRebindStatusSpec) {
	ctest.AssertResource(suite, expected.PCIID, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(expected, *status.TypedSpec())
	})
}

// A device already on its target driver is not unbound and probed again: for a device a VFIO user
// holds, that would hot-unplug it from the user.
func (suite *PCIDriverRebindSuite) TestBoundDeviceIsNotRebound() {
	suite.device(rebindDevice, "vfio-pci", "7")
	suite.openVFIOGroup()
	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseAdopted,
	})

	suite.Assert().Empty(suite.unbound("vfio-pci"))
	suite.Assert().Empty(suite.read("bus", "pci", "drivers_probe"))
	suite.Assert().Equal("vfio-pci", suite.override(rebindDevice))

	// A change to another device's configuration leaves this one alone too.
	suite.device(rebindOtherDevice, "nvme", "8")
	suite.configure(rebindOtherDevice)

	ctest.AssertResource(suite, rebindOtherDevice, func(*hardware.PCIDriverRebindStatus, *assert.Assertions) {})

	suite.Assert().Empty(suite.unbound("vfio-pci"))
	suite.Assert().Equal(rebindOtherDevice, suite.unbound("nvme"))
}

// A device on its host driver is unbound and probed for its target driver; the status follows the
// driver the kernel actually bound.
func (suite *PCIDriverRebindSuite) TestDeviceIsRebound() {
	suite.device(rebindDevice, "nvme", "7")
	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "nvme", Phase: hardware.PCIDriverRebindPhaseUnbound,
		Reason: `device is bound to "nvme", ensure the vfio-pci driver is available (its kernel module is loaded)`,
	})

	suite.Assert().Equal("vfio-pci", suite.override(rebindDevice))
	suite.Assert().Equal(rebindDevice, suite.unbound("nvme"))
	suite.Assert().Equal(rebindDevice, suite.read("bus", "pci", "drivers_probe"))

	// The kernel binds the device to the override once vfio-pci is available.
	suite.bind(rebindDevice, "vfio-pci")
	suite.reconcile()

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseBound,
	})
}

// A device which re-enumerates comes back without its override and on its host driver; the next
// reconcile brings it back to its target driver.
func (suite *PCIDriverRebindSuite) TestReenumeratedDeviceIsRebound() {
	suite.device(rebindDevice, "vfio-pci", "7")
	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseAdopted,
	})

	suite.Require().NoError(os.WriteFile(filepath.Join(suite.devicePath(rebindDevice), "driver_override"), []byte("(null)\n"), 0o644))
	suite.bind(rebindDevice, "nvme")
	suite.reconcile()

	suite.Require().Eventually(func() bool {
		return suite.unbound("nvme") == rebindDevice && suite.override(rebindDevice) == "vfio-pci"
	}, 5*time.Second, 10*time.Millisecond)
}

// moveToVFIO configures a device on nvme for vfio-pci, and lets the kernel bind it, so the controller
// has moved it.
func (suite *PCIDriverRebindSuite) moveToVFIO() {
	suite.device(rebindDevice, "nvme", "7")
	suite.configure(rebindDevice)

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(hardware.PCIDriverRebindPhaseUnbound, status.TypedSpec().Phase)
	})

	suite.bind(rebindDevice, "vfio-pci")
	suite.reconcile()

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseBound,
	})
}

// removeDevice makes a device the controller moved disappear, as on a re-enumeration, and checks it
// is still the controller's to hand back.
func (suite *PCIDriverRebindSuite) removeDevice() {
	suite.Require().NoError(os.RemoveAll(suite.devicePath(rebindDevice)))
	suite.reconcile()

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(hardware.PCIDriverRebindPhaseUnbound, status.TypedSpec().Phase)
		asrt.Empty(status.TypedSpec().Driver)
		asrt.Contains(status.TypedSpec().Reason, "device is not present")
	})
}

// A moved device which comes back on its target driver is still the controller's: removing its
// configuration hands it back.
func (suite *PCIDriverRebindSuite) TestMovedDeviceBackOnTargetIsReleased() {
	suite.moveToVFIO()
	suite.removeDevice()

	suite.device(rebindDevice, "vfio-pci", "7")
	suite.reconcile()

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseBound,
	})

	suite.Destroy(hardware.NewPCIDriverRebindConfig(rebindDevice))

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Equal(rebindDevice, suite.unbound("vfio-pci"))
	suite.Assert().Equal("\n", suite.override(rebindDevice))
}

// A moved device which comes back on its default driver, without the override, has nothing left to
// hand back: removing its configuration does not unbind it.
func (suite *PCIDriverRebindSuite) TestMovedDeviceBackOnDefaultIsNotUnbound() {
	suite.moveToVFIO()
	suite.removeDevice()

	suite.Require().NoError(os.WriteFile(filepath.Join(suite.sysfs, "bus", "pci", "drivers", "nvme", "unbind"), nil, 0o644))
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.sysfs, "bus", "pci", "drivers_probe"), nil, 0o644))
	suite.device(rebindDevice, "nvme", "7")

	suite.Destroy(hardware.NewPCIDriverRebindConfig(rebindDevice))

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Empty(suite.unbound("nvme"))
	suite.Assert().Empty(suite.read("bus", "pci", "drivers_probe"))
	suite.Assert().Equal("(null)\n", suite.override(rebindDevice))
}

// A moved device whose override was cleared by hand, while it stays on the driver the controller put
// it on, is still handed back once its configuration is removed.
func (suite *PCIDriverRebindSuite) TestMovedDeviceWithClearedOverrideIsReleased() {
	suite.moveToVFIO()

	suite.Require().NoError(os.WriteFile(filepath.Join(suite.devicePath(rebindDevice), "driver_override"), []byte("(null)\n"), 0o644))

	suite.Destroy(hardware.NewPCIDriverRebindConfig(rebindDevice))

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Equal(rebindDevice, suite.unbound("vfio-pci"))
	suite.Assert().Equal("\n", suite.override(rebindDevice))
}

// A device the controller moved, and which a VFIO user holds, is neither moved to another target nor
// handed back once its configuration is removed, until the user lets go of it.
func (suite *PCIDriverRebindSuite) TestMovedDeviceInUseIsReleasedOnceFree() {
	suite.moveToVFIO()
	suite.openVFIOGroup()

	ctest.UpdateWithConflicts(suite, hardware.NewPCIDriverRebindConfig(rebindDevice), func(cfg *hardware.PCIDriverRebindConfig) error {
		cfg.TypedSpec().TargetDriver = "e1000"

		return nil
	})

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "e1000", Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseReleasing,
		Reason: "held: device is in use by a VFIO user (pid 4242), stop it before changing the driver",
	})

	suite.Destroy(hardware.NewPCIDriverRebindConfig(rebindDevice))

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseReleasing,
		Reason: "held: device is in use by a VFIO user (pid 4242), stop it before changing the driver",
	})

	suite.Assert().Empty(suite.unbound("vfio-pci"))
	suite.Assert().Equal("vfio-pci", suite.override(rebindDevice))

	// The user exits: the device is handed back to its default driver.
	suite.Require().NoError(os.RemoveAll(filepath.Join(suite.procfs, "4242")))
	suite.reconcile()

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Equal(rebindDevice, suite.unbound("vfio-pci"))
	suite.Assert().Equal("\n", suite.override(rebindDevice))
}

// A device already on its target driver when its configuration appeared was never moved by the
// controller: removing the configuration leaves it on that driver, even once nothing holds it.
func (suite *PCIDriverRebindSuite) TestAdoptedDeviceIsNotReleased() {
	suite.device(rebindDevice, "vfio-pci", "7")
	suite.openVFIOGroup()
	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseAdopted,
	})

	suite.Destroy(hardware.NewPCIDriverRebindConfig(rebindDevice))

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Require().NoError(os.RemoveAll(filepath.Join(suite.procfs, "4242")))
	suite.reconcile()

	suite.Assert().Empty(suite.unbound("vfio-pci"))
	suite.Assert().Equal("\n", suite.override(rebindDevice))
}

// A target which is the driver a device already has does not make it the controller's to release.
func (suite *PCIDriverRebindSuite) TestTargetIsCurrentDriverIsNotReleased() {
	suite.device(rebindDevice, "nvme", "7")

	cfg := hardware.NewPCIDriverRebindConfig(rebindDevice)
	cfg.TypedSpec().PCIID = rebindDevice
	cfg.TypedSpec().TargetDriver = "nvme"
	suite.Create(cfg)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "nvme", Driver: "nvme", Phase: hardware.PCIDriverRebindPhaseAdopted,
	})

	suite.Destroy(cfg)

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Empty(suite.unbound("nvme"))
	suite.Assert().Empty(suite.read("bus", "pci", "drivers_probe"))
}

// A device the controller moved, but which its target driver did not take, has no driver: removing
// its configuration hands it back to its default driver.
func (suite *PCIDriverRebindSuite) TestUnboundDeviceIsReturned() {
	suite.device(rebindDevice, "nvme", "7")
	suite.configure(rebindDevice)

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(hardware.PCIDriverRebindPhaseUnbound, status.TypedSpec().Phase)
	})

	// The kernel left the device without a driver.
	suite.Require().NoError(os.Remove(filepath.Join(suite.devicePath(rebindDevice), "driver")))
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.sysfs, "bus", "pci", "drivers_probe"), nil, 0o644))

	suite.Destroy(hardware.NewPCIDriverRebindConfig(rebindDevice))

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Equal("\n", suite.override(rebindDevice))
	suite.Assert().Equal(rebindDevice, suite.read("bus", "pci", "drivers_probe"))
}

// VFIO variant drivers hand devices to VFIO users like vfio-pci: a device on one of them, held
// through its group, is not unbound.
func (suite *PCIDriverRebindSuite) TestVariantDriverInUseIsHeld() {
	suite.device(rebindDevice, "mlx5_vfio_pci", "7")
	suite.openVFIOGroup()
	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "mlx5_vfio_pci", Phase: hardware.PCIDriverRebindPhaseHeld,
		Reason: "held: device is in use by a VFIO user (pid 4242), stop it before changing the driver",
	})

	suite.Assert().Empty(suite.unbound("mlx5_vfio_pci"))
}

// retargetHeldByDeviceFD moves a device on vfio-pci to another driver while a process holds a VFIO
// device file descriptor naming the given device (none when empty), and reports whether the
// controller held the device rather than unbinding it.
func (suite *PCIDriverRebindSuite) retargetHeldByDeviceFD(fdDevice string) bool {
	suite.device(rebindDevice, "vfio-pci", "7")
	suite.openVFIODevice("4343", fdDevice)

	cfg := hardware.NewPCIDriverRebindConfig(rebindDevice)
	cfg.TypedSpec().PCIID = rebindDevice
	cfg.TypedSpec().TargetDriver = "nvme"
	suite.Create(cfg)

	var held bool

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.NotEmpty(status.TypedSpec().Reason)
		held = strings.Contains(status.TypedSpec().Reason, "in use by a VFIO user (pid 4343)")
	})

	return held && suite.unbound("vfio-pci") == ""
}

// A process may close a device's group and keep the device file descriptor it got from it: the
// device stays held, as unbinding it would block until the process lets it go.
func (suite *PCIDriverRebindSuite) TestDeviceFDInUseIsHeld() {
	suite.Assert().True(suite.retargetHeldByDeviceFD(rebindDevice))
}

// A kernel which does not name the device of a VFIO device file descriptor leaves it unknown: the
// device is held rather than risk a blocking unbind.
func (suite *PCIDriverRebindSuite) TestUnnamedDeviceFDIsHeld() {
	suite.Assert().True(suite.retargetHeldByDeviceFD(""))
}

// A VFIO device file descriptor for another device does not hold this one.
func (suite *PCIDriverRebindSuite) TestOtherDeviceFDIsNotHeld() {
	suite.device(rebindDevice, "vfio-pci", "7")
	suite.openVFIODevice("4343", rebindOtherDevice)

	cfg := hardware.NewPCIDriverRebindConfig(rebindDevice)
	cfg.TypedSpec().PCIID = rebindDevice
	cfg.TypedSpec().TargetDriver = "nvme"
	suite.Create(cfg)

	suite.Require().Eventually(func() bool {
		return suite.unbound("vfio-pci") == rebindDevice
	}, 5*time.Second, 10*time.Millisecond)
}

// A device which is not present is reported, not a controller failure, and does not hold back
// other devices.
func (suite *PCIDriverRebindSuite) TestAbsentDeviceIsReported() {
	suite.device(rebindOtherDevice, "nvme", "8")
	suite.configure(rebindDevice)
	suite.configure(rebindOtherDevice)

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(hardware.PCIDriverRebindPhasePending, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Reason, "device is not present")
	})

	suite.Require().Eventually(func() bool {
		return suite.unbound("nvme") == rebindOtherDevice
	}, 5*time.Second, 10*time.Millisecond)

	require.Equal(suite.T(), "vfio-pci", suite.override(rebindOtherDevice))
}

// disk lays out a fake disk of a PCI device with one partition, and publishes it.
func (suite *PCIDriverRebindSuite) disk(id, name string) {
	path := filepath.Join(suite.sysfs, "class", "block", name)
	suite.Require().NoError(os.MkdirAll(filepath.Join(path, "holders"), 0o755))
	suite.Require().NoError(os.WriteFile(filepath.Join(path, "dev"), []byte("259:0\n"), 0o644))

	partition := filepath.Join(path, name+"p1")
	suite.Require().NoError(os.MkdirAll(filepath.Join(partition, "holders"), 0o755))
	suite.Require().NoError(os.WriteFile(filepath.Join(partition, "partition"), []byte("1\n"), 0o644))
	suite.Require().NoError(os.WriteFile(filepath.Join(partition, "dev"), []byte("259:1\n"), 0o644))

	disk := block.NewDisk(block.NamespaceName, name)
	disk.TypedSpec().DevPath = "/dev/" + name
	disk.TypedSpec().BusPath = "/pci0000:00/0000:00:01.0/" + id + "/nvme"
	suite.Create(disk)
}

// A storage device is not taken from its host driver while another block device (e.g. an activated
// LVM volume group) holds one of its disks; the rebind goes ahead once it is released.
func (suite *PCIDriverRebindSuite) TestDiskInUseIsHeld() {
	suite.device(rebindDevice, "nvme", "7")
	suite.disk(rebindDevice, "nvme0n1")

	holder := filepath.Join(suite.sysfs, "class", "block", "nvme0n1", "nvme0n1p1", "holders", "dm-3")
	suite.Require().NoError(os.WriteFile(holder, nil, 0o644))

	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "nvme", Phase: hardware.PCIDriverRebindPhaseHeld,
		Reason: "held: nvme0n1p1 is in use by dm-3, release it before changing the driver",
	})

	suite.Assert().Empty(suite.unbound("nvme"))

	suite.Require().NoError(os.Remove(holder))
	suite.reconcile()

	suite.Require().Eventually(func() bool {
		return suite.unbound("nvme") == rebindDevice
	}, 5*time.Second, 10*time.Millisecond)
}

// A storage device is not taken from its host driver while one of its disks is mounted.
func (suite *PCIDriverRebindSuite) TestMountedDiskIsHeldAndNotReleased() {
	suite.device(rebindDevice, "nvme", "7")
	suite.disk(rebindDevice, "nvme0n1")

	suite.Require().NoError(os.MkdirAll(filepath.Join(suite.procfs, "self"), 0o755))
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.procfs, "self", "mountinfo"),
		[]byte("36 25 259:1 / /var/mnt/data rw,relatime shared:1 - xfs /dev/nvme0n1p1 rw\n"), 0o644))

	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "nvme", Phase: hardware.PCIDriverRebindPhaseHeld,
		Reason: "held: nvme0n1p1 is mounted, unmount it before changing the driver",
	})

	suite.Assert().Empty(suite.unbound("nvme"))

	// The configuration is removed while the bind is refused: the controller never moved the device,
	// so it is left on its host driver rather than unbound under the mounted filesystem.
	suite.Destroy(hardware.NewPCIDriverRebindConfig(rebindDevice))

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Empty(suite.unbound("nvme"))
	suite.Assert().Empty(suite.read("bus", "pci", "drivers_probe"))
}

// The namespace head of native NVMe multipath is found through its paths: a volume held on it holds
// the device it is reached through.
func (suite *PCIDriverRebindSuite) TestMultipathDiskInUseIsHeld() {
	suite.device(rebindDevice, "nvme", "7")

	head := filepath.Join(suite.sysfs, "devices", "virtual", "nvme-subsystem", "nvme-subsys0", "nvme0n1")
	path := filepath.Join(suite.devicePath(rebindDevice), "nvme", "nvme0", "nvme0c0n1")

	suite.Require().NoError(os.MkdirAll(filepath.Join(head, "holders"), 0o755))
	suite.Require().NoError(os.MkdirAll(filepath.Join(head, "multipath"), 0o755))
	suite.Require().NoError(os.MkdirAll(path, 0o755))
	suite.Require().NoError(os.Symlink(path, filepath.Join(head, "multipath", "nvme0c0n1")))
	suite.Require().NoError(os.WriteFile(filepath.Join(head, "holders", "dm-0"), nil, 0o644))
	suite.Require().NoError(os.MkdirAll(filepath.Join(suite.sysfs, "class", "block"), 0o755))
	suite.Require().NoError(os.Symlink(head, filepath.Join(suite.sysfs, "class", "block", "nvme0n1")))

	disk := block.NewDisk(block.NamespaceName, "nvme0n1")
	disk.TypedSpec().DevPath = "/dev/nvme0n1"
	disk.TypedSpec().BusPath = "/virtual/nvme-subsystem"
	suite.Create(disk)

	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "nvme", Phase: hardware.PCIDriverRebindPhaseHeld,
		Reason: "held: nvme0n1 is in use by dm-0, release it before changing the driver",
	})

	suite.Assert().Empty(suite.unbound("nvme"))
}
