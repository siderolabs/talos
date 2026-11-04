// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hardwarectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
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
	kernel        *fakeKernel
	devices       *fakeDevices
}

func TestHostDeviceNumber(t *testing.T) {
	t.Parallel()

	f, err := os.Open("/dev/null")
	require.NoError(t, err)

	t.Cleanup(func() { require.NoError(t, f.Close()) })

	var st unix.Stat_t

	require.NoError(t, unix.Stat("/dev/null", &st))

	// Read through the process's fd link, as the controller does.
	number, ok, err := hardwarectrl.HostDeviceNumber(fmt.Sprintf("/proc/self/fd/%d", f.Fd()))
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, hardwarectrl.NewDeviceNumber(st.Rdev, false), number)

	_, ok, err = hardwarectrl.HostDeviceNumber(t.TempDir())
	require.NoError(t, err)
	assert.False(t, ok)

	_, _, err = hardwarectrl.HostDeviceNumber(filepath.Join(t.TempDir(), "missing"))
	assert.ErrorIs(t, err, fs.ErrNotExist)
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

	suite.kernel = &fakeKernel{
		sysfs:    suite.sysfs,
		defaults: map[string]string{},
		loaded:   map[string]struct{}{},
		unbound:  map[string]string{},
	}

	suite.devices = &fakeDevices{chars: map[string]uint64{}}

	suite.reconcileCh = make(chan struct{}, 1)

	// Tests of a controller restart lay out what the previous controller left before starting it.
	if !strings.Contains(suite.T().Name(), "/TestRestart") {
		suite.start()
	}
}

// start registers the controller.
func (suite *PCIDriverRebindSuite) start() {
	ctrl := &hardwarectrl.PCIDriverRebindController{
		SysfsPath:   suite.sysfs,
		ProcfsPath:  suite.procfs,
		ReconcileCh: suite.reconcileCh,
	}
	ctrl.SetWriteSysfs(suite.kernel.write)
	ctrl.SetDeviceAccess(suite.devices)

	suite.Require().NoError(suite.Runtime().RegisterController(ctrl))
}

// fakeKernel acts on the fake sysfs as the kernel does on the controller's writes: an unbind leaves
// the device without a driver, and a probe binds a device without one to the driver its override
// names, once that driver is loaded, or else to its default driver. It records the last device
// unbound from each driver, and the last device probed.
type fakeKernel struct {
	mu sync.Mutex

	sysfs    string
	defaults map[string]string
	loaded   map[string]struct{}
	unbound  map[string]string
	probed   string
	// failUnbind, when set, is what an unbind fails with, after the device left its driver.
	failUnbind error
}

func (k *fakeKernel) write(path string, contents []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	id := strings.TrimSpace(string(contents))

	switch {
	case filepath.Base(path) == "unbind":
		link := filepath.Dir(path)

		driver, err := filepath.EvalSymlinks(link)
		if err != nil {
			return err
		}

		k.unbound[filepath.Base(driver)] = id

		if err := os.Remove(link); err != nil {
			return err
		}

		return k.failUnbind
	case path == filepath.Join(k.sysfs, "bus", "pci", "drivers_probe"):
		k.probed = id

		return k.probe(id)
	default:
		// Writing a sysfs attribute never creates it.
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
		if err != nil {
			return err
		}

		if _, err = f.Write(contents); err != nil {
			f.Close() //nolint:errcheck

			return err
		}

		return f.Close()
	}
}

func (k *fakeKernel) probe(id string) error {
	device := filepath.Join(k.sysfs, "bus", "pci", "devices", id)

	if _, err := os.Lstat(filepath.Join(device, "driver")); err == nil {
		return nil
	}

	driver := k.defaults[id]

	if contents, err := os.ReadFile(filepath.Join(device, "driver_override")); err == nil {
		if override := strings.TrimSpace(string(contents)); override != "" && override != "(null)" {
			if _, ok := k.loaded[override]; !ok {
				return nil
			}

			driver = override
		}
	}

	if driver == "" {
		return nil
	}

	return k.bind(id, driver)
}

// bind points a device's driver link at the given driver, replacing the link in one step.
func (k *fakeKernel) bind(id, driver string) error {
	drv := filepath.Join(k.sysfs, "bus", "pci", "drivers", driver)
	if err := os.MkdirAll(drv, 0o755); err != nil {
		return err
	}

	k.loaded[driver] = struct{}{}

	device := filepath.Join(k.sysfs, "bus", "pci", "devices", id)
	link := filepath.Join(device, ".driver")

	if err := os.RemoveAll(link); err != nil {
		return err
	}

	if err := os.Symlink(drv, link); err != nil {
		return err
	}

	return os.Rename(link, filepath.Join(device, "driver"))
}

// fakeDevices stands in for /dev in reading the numbers of device files: the character devices are
// listed by path, and a path, or the target of a /proc fd link, which is not listed is no device.
type fakeDevices struct {
	mu sync.Mutex

	// chars are the numbers of character device files, by path.
	chars map[string]uint64
}

func (f *fakeDevices) DeviceNumber(path string) (hardwarectrl.DeviceNumber, bool, error) {
	if target, err := os.Readlink(path); err == nil {
		path = target
	}

	f.mu.Lock()
	rdev, ok := f.chars[path]
	f.mu.Unlock()

	if !ok {
		return hardwarectrl.DeviceNumber{}, false, nil
	}

	return hardwarectrl.NewDeviceNumber(rdev, false), true, nil
}

// charDevice makes a path a character device file with the given number.
func (suite *PCIDriverRebindSuite) charDevice(path string, rdev uint64) {
	suite.devices.mu.Lock()
	defer suite.devices.mu.Unlock()

	suite.devices.chars[path] = rdev
}

// openFile makes a fake process hold a file open.
func (suite *PCIDriverRebindSuite) openFile(pid, target string) {
	fd := filepath.Join(suite.procfs, pid, "fd")
	suite.Require().NoError(os.MkdirAll(fd, 0o755))
	suite.Require().NoError(os.Symlink(target, filepath.Join(fd, "3")))
}

// device lays out a fake PCI device in an IOMMU group, bound to the given driver, its default. As
// in the kernel's sysfs, the device is under /sys/devices, and /sys/bus/pci/devices links to it.
func (suite *PCIDriverRebindSuite) device(id, driver, group string) {
	path := suite.deviceSyspath(id)
	suite.Require().NoError(os.RemoveAll(path))
	suite.Require().NoError(os.MkdirAll(path, 0o755))
	suite.Require().NoError(os.RemoveAll(suite.devicePath(id)))
	suite.Require().NoError(os.Symlink(path, suite.devicePath(id)))
	suite.Require().NoError(os.WriteFile(filepath.Join(path, "driver_override"), []byte("(null)\n"), 0o644))

	groupPath := filepath.Join(suite.sysfs, "kernel", "iommu_groups", group)
	suite.Require().NoError(os.MkdirAll(groupPath, 0o755))
	suite.Require().NoError(os.Symlink(groupPath, filepath.Join(path, "iommu_group")))

	suite.kernel.mu.Lock()
	suite.kernel.defaults[id] = driver
	suite.kernel.mu.Unlock()

	suite.bind(id, driver)
}

// bind binds a fake device to the given driver, as the kernel does on a probe.
func (suite *PCIDriverRebindSuite) bind(id, driver string) {
	suite.kernel.mu.Lock()
	defer suite.kernel.mu.Unlock()

	suite.Require().NoError(suite.kernel.bind(id, driver))
}

// unbound returns the last device unbound from a driver.
func (suite *PCIDriverRebindSuite) unbound(driver string) string {
	suite.kernel.mu.Lock()
	defer suite.kernel.mu.Unlock()

	return suite.kernel.unbound[driver]
}

// probed returns the last device probed.
func (suite *PCIDriverRebindSuite) probed() string {
	suite.kernel.mu.Lock()
	defer suite.kernel.mu.Unlock()

	return suite.kernel.probed
}

// forgetWrites clears the unbinds and probes recorded so far.
func (suite *PCIDriverRebindSuite) forgetWrites() {
	suite.kernel.mu.Lock()
	defer suite.kernel.mu.Unlock()

	suite.kernel.unbound = map[string]string{}
	suite.kernel.probed = ""
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
		// The kernel names the device by its path under /sys/devices.
		info += "vfio-device-syspath: /sys/devices/pci0000:00/" + id + "\n"
	}

	suite.Require().NoError(os.WriteFile(filepath.Join(fdinfo, "43"), []byte(info), 0o644))
}

func (suite *PCIDriverRebindSuite) devicePath(id string) string {
	return filepath.Join(suite.sysfs, "bus", "pci", "devices", id)
}

// deviceSyspath returns where a fake device is under /sys/devices.
func (suite *PCIDriverRebindSuite) deviceSyspath(id string) string {
	return filepath.Join(suite.sysfs, "devices", "pci0000:00", id)
}

// override returns the driver override of a device, or an empty string when it cannot be read. It
// does not fail the test itself, so that it is safe in conditions polled from another goroutine.
func (suite *PCIDriverRebindSuite) override(id string) string {
	contents, err := os.ReadFile(filepath.Join(suite.devicePath(id), "driver_override"))
	if err != nil {
		return ""
	}

	return string(contents)
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
	suite.Assert().Empty(suite.probed())
	suite.Assert().Equal("vfio-pci", suite.override(rebindDevice))

	// A change to another device's configuration leaves this one alone too.
	suite.device(rebindOtherDevice, "nvme", "8")
	suite.configure(rebindOtherDevice)

	suite.Require().Eventually(func() bool {
		return suite.unbound("nvme") == rebindOtherDevice
	}, 5*time.Second, 10*time.Millisecond)

	suite.Assert().Empty(suite.unbound("vfio-pci"))
}

// A device on its host driver is unbound and probed for its target driver; the status follows the
// driver the kernel actually bound.
func (suite *PCIDriverRebindSuite) TestDeviceIsRebound() {
	suite.device(rebindDevice, "nvme", "7")
	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseUnbound,
		Reason: "device is bound to no driver, ensure the vfio-pci driver is available (its kernel module is loaded)",
	})

	suite.Assert().Equal("vfio-pci", suite.override(rebindDevice))
	suite.Assert().Equal(rebindDevice, suite.unbound("nvme"))
	suite.Assert().Equal(rebindDevice, suite.probed())

	// The kernel binds the device to the override once vfio-pci is loaded.
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
	suite.kernel.mu.Lock()
	suite.Require().NoError(os.RemoveAll(suite.devicePath(rebindDevice)))
	suite.kernel.mu.Unlock()
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

// A moved device which went missing, and came back on its default driver without the override before
// the controller moved it again, has nothing left to hand back once its configuration is removed: it
// is not unbound. The state is laid out before the controller starts, as a reconcile while the
// configuration still exists would move the device again.
func (suite *PCIDriverRebindSuite) TestRestartMovedDeviceBackOnDefaultIsNotUnbound() {
	status := hardware.NewPCIDriverRebindStatus(rebindDevice)
	*status.TypedSpec() = hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseUnbound, Reason: "device is not present",
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), status, state.WithCreateOwner((&hardwarectrl.PCIDriverRebindController{}).Name())))

	suite.device(rebindDevice, "nvme", "7")
	suite.start()

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Empty(suite.unbound("nvme"))
	suite.Assert().Empty(suite.probed())
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
	suite.Assert().Empty(suite.probed())
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
	suite.Require().NoFileExists(filepath.Join(suite.devicePath(rebindDevice), "driver"))
	suite.forgetWrites()

	suite.Destroy(hardware.NewPCIDriverRebindConfig(rebindDevice))

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Equal("\n", suite.override(rebindDevice))
	suite.Assert().Equal(rebindDevice, suite.probed())
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

// recordMove leaves the status of a move from nvme which the controller recorded, but did not see
// through as it restarted.
func (suite *PCIDriverRebindSuite) recordMove() {
	status := hardware.NewPCIDriverRebindStatus(rebindDevice)
	*status.TypedSpec() = hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "nvme", Phase: hardware.PCIDriverRebindPhaseMoving,
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), status, state.WithCreateOwner((&hardwarectrl.PCIDriverRebindController{}).Name())))
}

// A move the controller recorded, and which completed before it restarted, still counts as a move:
// the device is Bound, not Adopted.
func (suite *PCIDriverRebindSuite) TestRestartAfterMoveIsBound() {
	suite.device(rebindDevice, "vfio-pci", "7")
	suite.recordMove()
	suite.configure(rebindDevice)
	suite.start()

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseBound,
	})
}

// A move the controller recorded, but which did not get as far as the unbind before it restarted,
// left the device on its driver: removing its configuration only clears the override.
func (suite *PCIDriverRebindSuite) TestRestartBeforeMoveDoesNotUnbind() {
	suite.device(rebindDevice, "nvme", "7")
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.devicePath(rebindDevice), "driver_override"), []byte("vfio-pci\n"), 0o644))
	suite.recordMove()
	suite.start()

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Empty(suite.unbound("nvme"))
	suite.Assert().Empty(suite.probed())
	suite.Assert().Equal("\n", suite.override(rebindDevice))
}

// A move is recorded before the device is touched: when that fails, the device is left alone.
func (suite *PCIDriverRebindSuite) TestMoveNotRecordedLeavesDevice() {
	suite.device(rebindDevice, "nvme", "7")

	// A status the controller cannot write over makes recording the move fail.
	status := hardware.NewPCIDriverRebindStatus(rebindDevice)
	status.TypedSpec().PCIID = rebindDevice
	suite.Require().NoError(suite.State().Create(suite.Ctx(), status, state.WithCreateOwner("another.Controller")))

	suite.configure(rebindDevice)

	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		suite.Require().Empty(suite.unbound("nvme"))
		suite.Require().Equal("(null)\n", suite.override(rebindDevice))
	}
}

// A VFIO group opened in no-IOMMU mode, through its noiommu node, holds its device.
func (suite *PCIDriverRebindSuite) TestNoIOMMUGroupInUseIsHeld() {
	suite.device(rebindDevice, "vfio-pci", "7")
	suite.openFile("4444", "/dev/vfio/noiommu-7")

	cfg := hardware.NewPCIDriverRebindConfig(rebindDevice)
	cfg.TypedSpec().PCIID = rebindDevice
	cfg.TypedSpec().TargetDriver = "nvme"
	suite.Create(cfg)

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(hardware.PCIDriverRebindPhaseHeld, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Reason, "in use by a VFIO user (pid 4444)")
	})

	suite.Assert().Empty(suite.unbound("vfio-pci"))
}

// A process whose descriptors cannot be read, other than because it exited, may hold the device.
func (suite *PCIDriverRebindSuite) TestUnreadableProcessHolds() {
	suite.device(rebindDevice, "vfio-pci", "7")

	// Listing the process's descriptors fails with ENOTDIR, which root cannot bypass either.
	suite.Require().NoError(os.MkdirAll(filepath.Join(suite.procfs, "4545"), 0o755))
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.procfs, "4545", "fd"), nil, 0o644))

	cfg := hardware.NewPCIDriverRebindConfig(rebindDevice)
	cfg.TypedSpec().PCIID = rebindDevice
	cfg.TypedSpec().TargetDriver = "nvme"
	suite.Create(cfg)

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(hardware.PCIDriverRebindPhaseHeld, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Reason, "cannot determine whether the device is in use")
	})

	suite.Assert().Empty(suite.unbound("vfio-pci"))
}

// A driver which cannot be read leaves the device's status as it was, and is retried.
func (suite *PCIDriverRebindSuite) TestUnreadableDriverKeepsStatus() {
	suite.moveToVFIO()

	// The driver link is replaced in one step: a reconcile in between would see no driver.
	unreadable := filepath.Join(suite.devicePath(rebindDevice), ".unreadable")
	suite.Require().NoError(os.WriteFile(unreadable, nil, 0o644))
	suite.Require().NoError(os.Rename(unreadable, filepath.Join(suite.devicePath(rebindDevice), "driver")))

	suite.reconcile()

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(hardware.PCIDriverRebindPhaseBound, status.TypedSpec().Phase)
		asrt.Equal("vfio-pci", status.TypedSpec().Driver)
		asrt.Contains(status.TypedSpec().Reason, "error reading bound driver")
	})

	suite.bind(rebindDevice, "vfio-pci")
	suite.reconcile()

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseBound,
	})
}

// A VFIO group node opened under another path (e.g. one a device plugin exposes in a container) is
// matched by its device number: it holds the device.
func (suite *PCIDriverRebindSuite) TestGroupOpenedUnderOtherPathIsHeld() {
	suite.device(rebindDevice, "vfio-pci", "7")
	suite.charDevice("/dev/vfio/7", unix.Mkdev(511, 7))
	suite.charDevice("/run/containers/dev/vfio/7", unix.Mkdev(511, 7))
	suite.openFile("4646", "/run/containers/dev/vfio/7")

	cfg := hardware.NewPCIDriverRebindConfig(rebindDevice)
	cfg.TypedSpec().PCIID = rebindDevice
	cfg.TypedSpec().TargetDriver = "nvme"
	suite.Create(cfg)

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(hardware.PCIDriverRebindPhaseHeld, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Reason, "in use by a VFIO user (pid 4646)")
	})

	suite.Assert().Empty(suite.unbound("vfio-pci"))
}

// An IOMMU group which cannot be read may be held: the device is not unbound.
func (suite *PCIDriverRebindSuite) TestUnreadableIOMMUGroupHolds() {
	suite.device(rebindDevice, "vfio-pci", "7")

	// Reading the link fails with EINVAL, which root cannot bypass either.
	group := filepath.Join(suite.devicePath(rebindDevice), "iommu_group")
	suite.Require().NoError(os.Remove(group))
	suite.Require().NoError(os.Mkdir(group, 0o755))

	cfg := hardware.NewPCIDriverRebindConfig(rebindDevice)
	cfg.TypedSpec().PCIID = rebindDevice
	cfg.TypedSpec().TargetDriver = "nvme"
	suite.Create(cfg)

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(hardware.PCIDriverRebindPhaseHeld, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Reason, "cannot determine whether the device is in use: error reading IOMMU group")
	})

	suite.Assert().Empty(suite.unbound("vfio-pci"))
}

// A device held through its iommufd node is not unbound.
func (suite *PCIDriverRebindSuite) TestIOMMUFDNodeInUseIsHeld() {
	suite.device(rebindDevice, "vfio-pci", "7")
	suite.Require().NoError(os.MkdirAll(filepath.Join(suite.devicePath(rebindDevice), "vfio-dev", "vfio0"), 0o755))
	suite.openFile("4747", "/dev/vfio/devices/vfio0")

	cfg := hardware.NewPCIDriverRebindConfig(rebindDevice)
	cfg.TypedSpec().PCIID = rebindDevice
	cfg.TypedSpec().TargetDriver = "nvme"
	suite.Create(cfg)

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(hardware.PCIDriverRebindPhaseHeld, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Reason, "in use by a VFIO user (pid 4747)")
	})

	suite.Assert().Empty(suite.unbound("vfio-pci"))
}

// An override which cannot be read keeps the status of a device being released, which is retried,
// rather than dropping it with the override left behind.
func (suite *PCIDriverRebindSuite) TestUnreadableOverrideKeepsRelease() {
	suite.device(rebindDevice, "vfio-pci", "7")
	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseAdopted,
	})

	// Reading the override fails with EISDIR.
	override := filepath.Join(suite.devicePath(rebindDevice), "driver_override")
	suite.Require().NoError(os.Remove(override))
	suite.Require().NoError(os.Mkdir(override, 0o755))

	suite.Destroy(hardware.NewPCIDriverRebindConfig(rebindDevice))

	ctest.AssertResource(suite, rebindDevice, func(status *hardware.PCIDriverRebindStatus, asrt *assert.Assertions) {
		asrt.Equal(hardware.PCIDriverRebindPhaseAdopted, status.TypedSpec().Phase)
		asrt.Contains(status.TypedSpec().Reason, "error reading driver override")
	})

	// The directory is swapped for a file in one step: a reconcile in between would find no override.
	readable := filepath.Join(suite.devicePath(rebindDevice), ".driver_override")
	suite.Require().NoError(os.WriteFile(readable, []byte("vfio-pci\n"), 0o644))
	suite.Require().NoError(unix.Renameat2(unix.AT_FDCWD, readable, unix.AT_FDCWD, override, unix.RENAME_EXCHANGE))
	suite.Require().NoError(os.RemoveAll(readable))
	suite.reconcile()

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Equal("\n", suite.override(rebindDevice))
	suite.Assert().Empty(suite.unbound("vfio-pci"))
}

// A recorded move which failed after the device left its driver (e.g. the unbind reported an error)
// leaves the device the controller's: Unbound, not Failed.
func (suite *PCIDriverRebindSuite) TestFailedUnbindAfterLeavingIsUnbound() {
	suite.kernel.mu.Lock()
	suite.kernel.failUnbind = unix.EIO
	suite.kernel.mu.Unlock()

	// Every status the device goes through is recorded: a later pass reports it again.
	events := make(chan state.Event, 64)
	suite.Require().NoError(suite.State().Watch(suite.Ctx(), hardware.NewPCIDriverRebindStatus(rebindDevice).Metadata(), events))

	suite.device(rebindDevice, "nvme", "7")
	suite.configure(rebindDevice)

	var phases []hardware.PCIDriverRebindPhase

	for {
		var event state.Event

		select {
		case event = <-events:
		case <-suite.Ctx().Done():
			suite.FailNow("no status reported the failed unbind", "phases seen: %v", phases)
		}

		status, ok := event.Resource.(*hardware.PCIDriverRebindStatus)
		if !ok {
			continue
		}

		phases = append(phases, status.TypedSpec().Phase)

		if strings.Contains(status.TypedSpec().Reason, "error unbinding device") {
			suite.Assert().Equal(hardware.PCIDriverRebindPhaseUnbound, status.TypedSpec().Phase)
			suite.Assert().Empty(status.TypedSpec().Driver)

			return
		}
	}
}

// A device without a driver has nothing to be unbound from: it is bound to its target even while a
// VFIO user holds its IOMMU group through another device of the group.
func (suite *PCIDriverRebindSuite) TestDriverlessDeviceIsBoundWhileGroupHeld() {
	suite.device(rebindOtherDevice, "vfio-pci", "7")
	suite.openVFIOGroup()

	suite.device(rebindDevice, "nvme", "7")

	suite.kernel.mu.Lock()
	suite.Require().NoError(os.Remove(filepath.Join(suite.devicePath(rebindDevice), "driver")))
	suite.kernel.mu.Unlock()

	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "vfio-pci", Phase: hardware.PCIDriverRebindPhaseBound,
	})
}
