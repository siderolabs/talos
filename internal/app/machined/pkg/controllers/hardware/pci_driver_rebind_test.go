// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
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

		return os.Remove(link)
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

// device lays out a fake PCI device in an IOMMU group, bound to the given driver, its default.
func (suite *PCIDriverRebindSuite) device(id, driver, group string) {
	path := suite.devicePath(id)
	suite.Require().NoError(os.MkdirAll(path, 0o755))
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
		info += "vfio-device-syspath: /sys/bus/pci/devices/" + id + "\n"
	}

	suite.Require().NoError(os.WriteFile(filepath.Join(fdinfo, "43"), []byte(info), 0o644))
}

func (suite *PCIDriverRebindSuite) devicePath(id string) string {
	return filepath.Join(suite.sysfs, "bus", "pci", "devices", id)
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

// disk lays out a fake disk of a PCI device with one partition in sysfs, as the kernel does; no
// Disk resource is published, as it may not exist yet.
func (suite *PCIDriverRebindSuite) disk(id, name string) {
	path := filepath.Join(suite.devicePath(id), "nvme", "nvme0", name)
	suite.Require().NoError(os.MkdirAll(filepath.Join(path, "holders"), 0o755))
	suite.Require().NoError(os.WriteFile(filepath.Join(path, "dev"), []byte("259:0\n"), 0o644))

	partition := filepath.Join(path, name+"p1")
	suite.Require().NoError(os.MkdirAll(filepath.Join(partition, "holders"), 0o755))
	suite.Require().NoError(os.WriteFile(filepath.Join(partition, "partition"), []byte("1\n"), 0o644))
	suite.Require().NoError(os.WriteFile(filepath.Join(partition, "dev"), []byte("259:1\n"), 0o644))

	classBlock := filepath.Join(suite.sysfs, "class", "block")
	suite.Require().NoError(os.MkdirAll(classBlock, 0o755))
	suite.Require().NoError(os.Symlink(path, filepath.Join(classBlock, name)))
	suite.Require().NoError(os.Symlink(partition, filepath.Join(classBlock, name+"p1")))
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
	suite.Assert().Empty(suite.probed())
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

	suite.configure(rebindDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "nvme", Phase: hardware.PCIDriverRebindPhaseHeld,
		Reason: "held: nvme0n1 is in use by dm-0, release it before changing the driver",
	})

	suite.Assert().Empty(suite.unbound("nvme"))
}

// class sets the PCI class code of a fake device, e.g. "0x010802" for an NVMe controller.
func (suite *PCIDriverRebindSuite) class(id, class string) {
	suite.Require().NoError(os.WriteFile(filepath.Join(suite.devicePath(id), "class"), []byte(class+"\n"), 0o644))
}

// A storage device is held while a block device cannot be placed yet, as it may be on that device;
// a device which is not a storage controller is not.
func (suite *PCIDriverRebindSuite) TestUnplacedBlockDeviceHoldsStorage() {
	// A device-mapper device whose slaves are not set up yet.
	dm := filepath.Join(suite.sysfs, "devices", "virtual", "block", "dm-0")
	suite.Require().NoError(os.MkdirAll(filepath.Join(dm, "dm"), 0o755))
	suite.Require().NoError(os.MkdirAll(filepath.Join(suite.sysfs, "class", "block"), 0o755))
	suite.Require().NoError(os.Symlink(dm, filepath.Join(suite.sysfs, "class", "block", "dm-0")))

	suite.device(rebindDevice, "nvme", "7")
	suite.class(rebindDevice, "0x010802")
	suite.device(rebindOtherDevice, "e1000", "8")
	suite.class(rebindOtherDevice, "0x020000")
	suite.configure(rebindDevice)
	suite.configure(rebindOtherDevice)

	suite.assertStatus(hardware.PCIDriverRebindStatusSpec{
		PCIID: rebindDevice, TargetDriver: "vfio-pci", Driver: "nvme", Phase: hardware.PCIDriverRebindPhaseHeld,
		Reason: "held: cannot tell yet whether dm-0 is on this device",
	})

	suite.Require().Eventually(func() bool {
		return suite.unbound("e1000") == rebindOtherDevice
	}, 5*time.Second, 10*time.Millisecond)

	suite.Assert().Empty(suite.unbound("nvme"))
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

// A device on a VFIO driver has no block devices on the host: an unplaced block device does not
// hold handing it back.
func (suite *PCIDriverRebindSuite) TestUnplacedBlockDeviceDoesNotHoldRelease() {
	suite.moveToVFIO()
	suite.class(rebindDevice, "0x010802")

	dm := filepath.Join(suite.sysfs, "devices", "virtual", "block", "dm-0")
	suite.Require().NoError(os.MkdirAll(filepath.Join(dm, "dm"), 0o755))
	suite.Require().NoError(os.MkdirAll(filepath.Join(suite.sysfs, "class", "block"), 0o755))
	suite.Require().NoError(os.Symlink(dm, filepath.Join(suite.sysfs, "class", "block", "dm-0")))

	suite.Destroy(hardware.NewPCIDriverRebindConfig(rebindDevice))

	ctest.AssertNoResource[*hardware.PCIDriverRebindStatus](suite, rebindDevice)

	suite.Assert().Equal(rebindDevice, suite.unbound("vfio-pci"))
}
