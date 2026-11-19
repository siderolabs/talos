// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/kobject"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// pciDriverRebindRetryInterval is how often devices which could not be brought to their target
// driver, or released, are retried while no other event arrives.
const pciDriverRebindRetryInterval = 10 * time.Second

// PCIDriverRebindController binds PCI devices to a specific driver and unbinds them from the host driver.
//
// A device already bound to its target driver is left alone. A device a VFIO user (e.g. a virtual
// machine) holds is never unbound, as unbinding makes the user release the device, hot-unplugging it
// from a running guest. Devices are reconciled again whenever a PCI device is added or bound, so a
// device which re-enumerates is brought back to its target driver.
//
// The phase of each device, kept in its PCIDriverRebindStatus, records whether the controller took
// it from its driver: once its configuration is removed, only such a device is handed back to its
// default driver, and a device it never moved is left on the driver it has. Keeping the phase in the
// status rather than in memory carries it over a controller restart.
type PCIDriverRebindController struct {
	V1Alpha1Mode v1alpha1runtime.Mode
	SysfsPath    string
	ProcfsPath   string
	// ReconcileCh triggers an additional reconcile for testing.
	ReconcileCh <-chan struct{}

	// reported is the last status reported per device, so that a retry does not log again.
	reported map[string]hardware.PCIDriverRebindStatusSpec

	// writeSysfs writes a sysfs attribute; tests replace it to act as the kernel does.
	writeSysfs func(path string, contents []byte) error
	// devices reads the numbers of device files; tests replace it.
	devices deviceAccess

	// open are the files the host's processes hold open, read from /proc when first needed in a
	// reconcile, and again after a device's driver changes.
	open *openFiles
}

// openFile is a file a process holds open, as /proc shows it.
type openFile struct {
	pid    int
	target string
	// number is set for a device file.
	number   deviceNumber
	isDevice bool
	// vfioSyspath names the device of a VFIO device file descriptor, when its fdinfo does.
	vfioSyspath string
}

// openFiles are the files the host's processes hold open, or why they could not be read.
type openFiles struct {
	files []openFile
	err   error
}

// deviceNumber is the number of a device file, and whether it is a block (rather than a character)
// device.
type deviceNumber struct {
	rdev  uint64
	block bool
}

// deviceAccess reads the numbers of device files.
type deviceAccess interface {
	// DeviceNumber returns the number of the device file at a path (following links, e.g. a /proc fd
	// link to the file a process opened), or false when it is no device file.
	DeviceNumber(path string) (deviceNumber, bool, error)
}

// hostDeviceAccess reads device files on the host.
type hostDeviceAccess struct{}

// DeviceNumber implements deviceAccess.
func (hostDeviceAccess) DeviceNumber(path string) (deviceNumber, bool, error) {
	var st unix.Statx_t

	// The file a process opened may be on any filesystem. AT_STATX_DONT_SYNC answers from cached
	// attributes, so a hung network or FUSE filesystem doesn't block the controller; the type and
	// number of a device file never change.
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_STATX_DONT_SYNC, unix.STATX_TYPE, &st); err != nil {
		return deviceNumber{}, false, err
	}

	rdev := unix.Mkdev(st.Rdev_major, st.Rdev_minor)

	switch uint32(st.Mode) & unix.S_IFMT {
	case unix.S_IFBLK:
		return deviceNumber{rdev: rdev, block: true}, true, nil
	case unix.S_IFCHR:
		return deviceNumber{rdev: rdev}, true, nil
	}

	return deviceNumber{}, false, nil
}

// Name implements controller.Controller interface.
func (c *PCIDriverRebindController) Name() string {
	return "hardware.PCIDriverRebindController"
}

// Inputs implements controller.Controller interface.
func (c *PCIDriverRebindController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (c *PCIDriverRebindController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hardware.PCIDriverRebindStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (c *PCIDriverRebindController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// Skip PCI rebind handling if running in a container or agent mode.
	if c.V1Alpha1Mode.InContainer() || c.V1Alpha1Mode.IsAgent() {
		return nil
	}

	if c.SysfsPath == "" {
		c.SysfsPath = "/sys"
	}

	if c.ProcfsPath == "" {
		c.ProcfsPath = "/proc"
	}

	if c.reported == nil {
		c.reported = map[string]hardware.PCIDriverRebindStatusSpec{}
	}

	if c.devices == nil {
		c.devices = hostDeviceAccess{}
	}

	if c.writeSysfs == nil {
		c.writeSysfs = func(path string, contents []byte) error {
			return os.WriteFile(path, contents, 0o200)
		}
	}

	// wait for udevd to be healthy, this is to ensure that host drivers if any are loaded.
	if err := runtimectrl.WaitForDevicesReady(ctx, r,
		[]controller.Input{
			{
				Namespace: hardware.NamespaceName,
				Type:      hardware.PCIDriverRebindConfigType,
				Kind:      controller.InputWeak,
			},
		}); err != nil {
		return fmt.Errorf("error waiting for devices to be ready: %w", err)
	}

	// A device which is removed and added again (e.g. a hot-plug slot reporting a link reset) comes
	// back without its driver override and is claimed by its host driver.
	watcher, err := kobject.NewWatcher(logger)
	if err != nil {
		return fmt.Errorf("error creating kobject watcher: %w", err)
	}
	defer watcher.Close() //nolint:errcheck

	return c.loop(ctx, r, logger, watcher)
}

// loop reconciles on configuration changes and PCI device uevents, and retries devices left
// pending.
//
//nolint:gocyclo
func (c *PCIDriverRebindController) loop(ctx context.Context, r controller.Runtime, logger *zap.Logger, watcher *kobject.Watcher) error {
	watchCh := watcher.Run("pci")
	rateLimitedTrigger := trigger.NewRateLimitedTrigger(ctx, r, 1, 1)

	retry := time.NewTicker(pciDriverRebindRetryInterval)
	defer retry.Stop()

	var (
		pending bool
		err     error
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-watchCh:
			if !ok {
				select {
				case err := <-watcher.ErrCh():
					return err
				default:
					return nil
				}
			}

			if ev.Action == kobject.ActionAdd || ev.Action == kobject.ActionBind || ev.Action == kobject.ActionUnbind {
				rateLimitedTrigger.QueueReconcile()
			}

			continue
		case err := <-watcher.ErrCh():
			return err
		case <-retry.C:
			if !pending {
				continue
			}
		case <-r.EventCh():
		case <-c.ReconcileCh:
		}

		if pending, err = c.reconcile(ctx, r, logger); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

// reconcile brings every configured device to its target driver, and hands back the devices
// whose configuration was removed. It reports whether any device is left to retry.
func (c *PCIDriverRebindController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (bool, error) {
	// What processes hold open is read again in each pass.
	c.forgetOpenFiles()

	configs, err := safe.ReaderListAll[*hardware.PCIDriverRebindConfig](ctx, r)
	if err != nil {
		return false, fmt.Errorf("error listing all PCI rebind configs: %w", err)
	}

	// The previous phase of each device, including those whose configuration was removed, says
	// whether the controller moved it.
	previousStatuses, err := safe.ReaderListAll[*hardware.PCIDriverRebindStatus](ctx, r)
	if err != nil {
		return false, fmt.Errorf("error listing PCI rebind statuses: %w", err)
	}

	previous := make(map[string]hardware.PCIDriverRebindStatusSpec, previousStatuses.Len())

	for status := range previousStatuses.All() {
		previous[status.Metadata().ID()] = *status.TypedSpec()
	}

	r.StartTrackingOutputs()

	configured, pending, err := c.bindConfigured(ctx, r, logger, configs, previous)
	if err != nil {
		return false, err
	}

	released, err := c.releaseRemoved(ctx, r, logger, configured, previous)
	if err != nil {
		return false, err
	}

	pending = pending || released

	return pending, safe.CleanupOutputs[*hardware.PCIDriverRebindStatus](ctx, r)
}

// bindConfigured brings every configured device to its target driver. It returns the configured
// devices, and whether any of them is left to retry.
func (c *PCIDriverRebindController) bindConfigured(ctx context.Context, r controller.Runtime, logger *zap.Logger,
	configs safe.List[*hardware.PCIDriverRebindConfig], previous map[string]hardware.PCIDriverRebindStatusSpec,
) (map[string]struct{}, bool, error) {
	var pending bool

	configured := make(map[string]struct{}, configs.Len())

	for cfg := range configs.All() {
		pciID, target := cfg.TypedSpec().PCIID, cfg.TypedSpec().TargetDriver
		configured[pciID] = struct{}{}

		// The move is recorded before the device is touched, so that a controller restarting halfway
		// through it still knows it took the device.
		record := func(status hardware.PCIDriverRebindStatusSpec) error {
			return c.writeStatus(ctx, r, logger, status)
		}

		status, err := c.bind(pciID, target, previous[pciID], record)
		if err != nil {
			return nil, false, err
		}

		// A device on its target with a reason (e.g. its driver could not be read) is retried too.
		pending = pending || !onTarget(status.Phase) || status.Reason != ""

		if err := c.writeStatus(ctx, r, logger, status); err != nil {
			return nil, false, err
		}
	}

	return configured, pending, nil
}

// releaseRemoved hands back the devices whose configuration was removed. It reports whether any of
// them is left to retry, as it cannot be handed back yet.
func (c *PCIDriverRebindController) releaseRemoved(ctx context.Context, r controller.Runtime, logger *zap.Logger,
	configured map[string]struct{}, previous map[string]hardware.PCIDriverRebindStatusSpec,
) (bool, error) {
	var pending bool

	for pciID, status := range previous {
		if _, ok := configured[pciID]; ok {
			continue
		}

		status, released := c.release(status)
		if released {
			delete(c.reported, pciID)

			logger.Info("PCI device set to default", zap.String("id", pciID))

			continue
		}

		// The device stays reported until it can be handed back.
		pending = true

		if err := c.writeStatus(ctx, r, logger, status); err != nil {
			return false, err
		}
	}

	return pending, nil
}

// onTarget reports whether a device in the given phase is on its target driver.
func onTarget(phase hardware.PCIDriverRebindPhase) bool {
	return phase == hardware.PCIDriverRebindPhaseBound || phase == hardware.PCIDriverRebindPhaseAdopted
}

// wasMoved reports whether the controller took a device, now on the given driver, from its driver.
// A move recorded but not seen through, as the controller restarted, only counts once the device
// left the driver it had.
func wasMoved(previous hardware.PCIDriverRebindStatusSpec, driver string) bool {
	if previous.Phase == hardware.PCIDriverRebindPhaseMoving {
		return driver != previous.Driver
	}

	return previous.Phase.Moved()
}

// bind brings a device to its target driver, unless it already is there, or is in use on its current
// driver. The phase it returns records whether the controller moved the device, carried over from
// the previous one while the device stays on the driver the controller put it on. The move is
// recorded before the device is touched; an error doing so is returned, and the device left alone.
//
//nolint:gocyclo
func (c *PCIDriverRebindController) bind(pciID, target string, previous hardware.PCIDriverRebindStatusSpec,
	record func(hardware.PCIDriverRebindStatusSpec) error,
) (hardware.PCIDriverRebindStatusSpec, error) {
	status := hardware.PCIDriverRebindStatusSpec{PCIID: pciID, TargetDriver: target}

	driver, err := c.boundDriver(pciID)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		// Nothing is known of the device: it keeps the status it had, and is retried.
		kept := previous
		kept.PCIID, kept.TargetDriver, kept.Reason = pciID, target, err.Error()

		return kept, nil
	}

	moved := wasMoved(previous, driver)

	// A failure which leaves the device where it is keeps the phase the device was in; a move
	// recorded but not seen through left the device off its driver.
	stay := func(reason string) (hardware.PCIDriverRebindStatusSpec, error) {
		status.Reason = reason
		status.Phase = hardware.PCIDriverRebindPhaseFailed

		if moved {
			status.Phase = previous.Phase

			if previous.Phase == hardware.PCIDriverRebindPhaseMoving {
				status.Phase = hardware.PCIDriverRebindPhaseUnbound
			}
		}

		return status, nil
	}

	if err != nil {
		status.Reason = err.Error()
		status.Phase = hardware.PCIDriverRebindPhasePending

		// A device the controller moved stays its to hand back, should it come back on the target driver.
		if moved {
			status.Phase = hardware.PCIDriverRebindPhaseUnbound
		}

		return status, nil
	}

	status.Driver = driver

	if driver == target {
		status.Phase = hardware.PCIDriverRebindPhaseAdopted

		if moved {
			status.Phase = hardware.PCIDriverRebindPhaseBound
		}

		// Kept in place so that a later re-probe of the device keeps the target driver; writing the
		// override does not unbind the device.
		if err := c.setOverride(pciID, target); err != nil {
			status.Reason = err.Error()
		}

		return status, nil
	}

	if reason := c.inUse(pciID, driver); reason != "" {
		status.Reason = reason
		status.Phase = hardware.PCIDriverRebindPhaseHeld

		// A device the controller moved, and which cannot be moved again yet (e.g. to another target),
		// is still the controller's to hand back.
		if moved {
			status.Phase = hardware.PCIDriverRebindPhaseReleasing
		}

		return status, nil
	}

	if !moved {
		status.Phase = hardware.PCIDriverRebindPhaseMoving

		if err := record(status); err != nil {
			return status, err
		}

		// From here on, a failure is judged as the recorded move is after a restart: the device is
		// the controller's once it left the driver it had.
		stay = func(reason string) (hardware.PCIDriverRebindStatusSpec, error) {
			return c.recordedMoveFailed(status, driver, reason), nil
		}
	}

	if err := c.setOverride(pciID, target); err != nil {
		return stay(err.Error())
	}

	if driver != "" {
		if err := c.unbind(pciID); err != nil {
			return stay(err.Error())
		}
	}

	return c.probeTarget(status), nil
}

// recordedMoveFailed reports a device whose move, recorded from the given driver, failed halfway: it
// is Unbound, still the controller's to hand back, if it left that driver nonetheless, and Failed if
// it did not. A device whose driver cannot be read stays Moving, to be judged on the next pass.
func (c *PCIDriverRebindController) recordedMoveFailed(status hardware.PCIDriverRebindStatusSpec, driver, reason string,
) hardware.PCIDriverRebindStatusSpec {
	status.Reason = reason

	current, err := c.boundDriver(status.PCIID)

	switch {
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		status.Phase, status.Driver = hardware.PCIDriverRebindPhaseMoving, driver
	case wasMoved(hardware.PCIDriverRebindStatusSpec{Phase: hardware.PCIDriverRebindPhaseMoving, Driver: driver}, current):
		status.Phase, status.Driver = hardware.PCIDriverRebindPhaseUnbound, current
	default:
		status.Phase, status.Driver = hardware.PCIDriverRebindPhaseFailed, driver
	}

	return status
}

// probeTarget probes a device which left the driver it had, and reports where it landed. From the
// unbind on, the device is the controller's to hand back: Unbound, or Bound once on its target.
func (c *PCIDriverRebindController) probeTarget(status hardware.PCIDriverRebindStatusSpec) hardware.PCIDriverRebindStatusSpec {
	status.Phase = hardware.PCIDriverRebindPhaseUnbound

	if err := c.probe(status.PCIID); err != nil {
		status.Reason = err.Error()

		return status
	}

	driver, err := c.boundDriver(status.PCIID)
	if err != nil {
		status.Reason = err.Error()

		return status
	}

	status.Driver = driver

	if driver != status.TargetDriver {
		status.Reason = fmt.Sprintf("device is bound to %s, ensure the %s driver is available (its kernel module is loaded)",
			driverName(driver), status.TargetDriver)

		return status
	}

	status.Phase = hardware.PCIDriverRebindPhaseBound

	return status
}

// release hands a device whose configuration was removed back to its default driver, if the
// controller moved it and it is no longer in use. A device the controller never moved keeps its
// driver: only the driver override the controller left is cleared.
func (c *PCIDriverRebindController) release(previous hardware.PCIDriverRebindStatusSpec) (hardware.PCIDriverRebindStatusSpec, bool) {
	pciID := previous.PCIID
	status := hardware.PCIDriverRebindStatusSpec{PCIID: pciID, Phase: previous.Phase}

	driver, err := c.boundDriver(pciID)
	if errors.Is(err, fs.ErrNotExist) {
		// The device is gone: there is nothing left to hand back.
		return status, true
	}

	if err != nil {
		// Nothing is known of the device: it keeps the driver it had.
		status.Driver, status.Reason = previous.Driver, err.Error()

		return status, false
	}

	status.Driver = driver

	if !wasMoved(previous, driver) {
		// Clearing the override does not unbind the device: it only stops a later probe from binding
		// it to the target the configuration named.
		set, err := c.overrideIsSet(pciID)
		if err == nil && set {
			err = c.setOverride(pciID, "\n")
		}

		if err != nil {
			status.Reason = err.Error()

			return status, false
		}

		return status, true
	}

	if c.backOnDefault(pciID, driver, previous) {
		return status, true
	}

	status.Phase = hardware.PCIDriverRebindPhaseReleasing

	if reason := c.inUse(pciID, driver); reason != "" {
		status.Reason = reason

		return status, false
	}

	// Writing a newline to driver_override clears it, so the device gets its default driver.
	if err := c.rebind(pciID, driver, "\n"); err != nil {
		status.Reason = err.Error()

		return status, false
	}

	return status, true
}

// inUse returns why a device must not be unbound from the driver it is on, or an empty string when
// it may be. A device without a driver has nothing to be unbound from.
func (c *PCIDriverRebindController) inUse(pciID, driver string) string {
	if driver == "" {
		return ""
	}

	return c.heldByVFIOUser(pciID)
}

// rebind sets the driver override of a device, unbinds it from its current driver, and probes it.
func (c *PCIDriverRebindController) rebind(pciID, driver, override string) error {
	if err := c.setOverride(pciID, override); err != nil {
		return err
	}

	if driver != "" {
		if err := c.unbind(pciID); err != nil {
			return err
		}
	}

	return c.probe(pciID)
}

// unbind unbinds a device from its current driver.
func (c *PCIDriverRebindController) unbind(pciID string) error {
	// Unbinding may take a while (e.g. an NVMe controller shutting down): devices checked after this
	// one read what processes hold open again.
	defer c.forgetOpenFiles()

	if err := c.writeSysfs(filepath.Join(c.devicePath(pciID), "driver", "unbind"), []byte(pciID)); err != nil &&
		!errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("error unbinding device with id: %s, %w", pciID, err)
	}

	return nil
}

// probe asks the kernel to bind a device to the driver its override, or its default, names.
func (c *PCIDriverRebindController) probe(pciID string) error {
	// As does probing, which waits for the driver to set the device up.
	defer c.forgetOpenFiles()

	if err := c.writeSysfs(filepath.Join(c.SysfsPath, "bus", "pci", "drivers_probe"), []byte(pciID)); err != nil {
		return fmt.Errorf("error probing driver for device with id: %s, %w", pciID, err)
	}

	return nil
}

// setOverride writes the driver override of a device, unless it is already set to the same driver.
func (c *PCIDriverRebindController) setOverride(pciID, override string) error {
	if strings.TrimSpace(override) != "" && c.overrideSet(pciID, override) {
		return nil
	}

	if err := c.writeSysfs(filepath.Join(c.devicePath(pciID), "driver_override"), []byte(override)); err != nil {
		return fmt.Errorf("error writing driver override for device with id: %s, target driver: %q, %w", pciID, strings.TrimSpace(override), err)
	}

	return nil
}

// overrideIsSet reports whether a device has a driver override.
func (c *PCIDriverRebindController) overrideIsSet(pciID string) (bool, error) {
	contents, err := os.ReadFile(filepath.Join(c.devicePath(pciID), "driver_override"))
	if err != nil {
		return false, fmt.Errorf("error reading driver override for device with id: %s, %w", pciID, err)
	}

	override := string(bytes.TrimSpace(contents))

	return override != "" && override != "(null)", nil
}

// backOnDefault reports whether a device the controller moved came back (e.g. re-enumerated) on a
// driver, without the override, which is neither the one it last had nor its target: it is on its
// default driver already, with nothing left to hand back.
func (c *PCIDriverRebindController) backOnDefault(pciID, driver string, previous hardware.PCIDriverRebindStatusSpec) bool {
	if driver == "" || !c.overrideCleared(pciID) {
		return false
	}

	return driver != previous.Driver || (previous.TargetDriver != "" && driver != previous.TargetDriver)
}

// overrideCleared reports whether a device's driver override could be read, and is not set.
func (c *PCIDriverRebindController) overrideCleared(pciID string) bool {
	contents, err := os.ReadFile(filepath.Join(c.devicePath(pciID), "driver_override"))
	if err != nil {
		return false
	}

	override := string(bytes.TrimSpace(contents))

	return override == "" || override == "(null)"
}

// overrideSet reports whether the driver override of a device is the given driver.
func (c *PCIDriverRebindController) overrideSet(pciID, driver string) bool {
	contents, err := os.ReadFile(filepath.Join(c.devicePath(pciID), "driver_override"))

	return err == nil && string(bytes.TrimSpace(contents)) == driver
}

// boundDriver returns the driver a device is bound to, or an empty string when it has none.
func (c *PCIDriverRebindController) boundDriver(pciID string) (string, error) {
	if _, err := os.Stat(c.devicePath(pciID)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("device is not present: %w", err)
		}

		return "", err
	}

	link, err := os.Readlink(filepath.Join(c.devicePath(pciID), "driver"))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("error reading bound driver for device with id: %s, %w", pciID, err)
	}

	return filepath.Base(link), nil
}

// heldByVFIOUser returns why a device a VFIO user holds must not be unbound, or an empty string
// when it may be.
//
// It does not depend on the device's driver: besides vfio-pci, VFIO variant drivers (e.g.
// mlx5_vfio_pci) hand devices to VFIO users the same way, and a device has VFIO nodes to hold only
// while one of them owns it.
func (c *PCIDriverRebindController) heldByVFIOUser(pciID string) string {
	pid, err := c.vfioUser(pciID)

	switch {
	case err != nil:
		// Unbinding a device which may be in use is what this check exists to prevent.
		return fmt.Sprintf("held: cannot determine whether the device is in use: %s", err)
	case pid != 0:
		return fmt.Sprintf("held: device is in use by a VFIO user (pid %d), stop it before changing the driver", pid)
	}

	return ""
}

// vfioUser returns the ID of a process holding a device through VFIO, or zero: through its group or
// iommufd node (by path, or by device number, as a node may be opened under another path, e.g. one a
// device plugin exposes in a container), or through a device file descriptor obtained from its
// group, which a process may keep after closing the group.
func (c *PCIDriverRebindController) vfioUser(pciID string) (int, error) {
	nodes, err := c.vfioNodes(pciID)
	if err != nil || len(nodes) == 0 {
		return 0, err
	}

	numbers, err := c.nodeNumbers(nodes)
	if err != nil {
		return 0, err
	}

	files, err := c.openFiles()
	if err != nil {
		return 0, err
	}

	syspath := c.deviceSyspath(pciID)

	for _, file := range files {
		switch {
		case slices.Contains(nodes, file.target),
			file.target == vfioDeviceFD && (file.vfioSyspath == "" || file.vfioSyspath == syspath),
			file.isDevice && slices.Contains(numbers, file.number):
			return file.pid, nil
		}
	}

	return 0, nil
}

// nodeNumbers returns the numbers of those of the given device nodes which exist on the host.
func (c *PCIDriverRebindController) nodeNumbers(nodes []string) ([]deviceNumber, error) {
	var numbers []deviceNumber

	for _, node := range nodes {
		number, ok, err := c.devices.DeviceNumber(node)

		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return nil, err
		case ok:
			numbers = append(numbers, number)
		}
	}

	return numbers, nil
}

// openFiles returns the files the host's processes hold open, read from /proc on first use in a
// reconcile, and again after a device's driver changes. A process which exits, or a descriptor closed, while it is read holds nothing; any
// other error reading /proc is returned, as a device may then be in use.
func (c *PCIDriverRebindController) openFiles() ([]openFile, error) {
	if c.open == nil {
		files, err := c.readOpenFiles()
		c.open = &openFiles{files: files, err: err}
	}

	return c.open.files, c.open.err
}

// forgetOpenFiles makes the next check read what processes hold open from /proc again.
func (c *PCIDriverRebindController) forgetOpenFiles() {
	c.open = nil
}

func (c *PCIDriverRebindController) readOpenFiles() ([]openFile, error) {
	procs, err := os.ReadDir(c.ProcfsPath)
	if err != nil {
		return nil, err
	}

	var files []openFile

	for _, proc := range procs {
		pid, err := strconv.Atoi(proc.Name())
		if err != nil {
			continue
		}

		files, err = c.readProcessFiles(pid, files)
		if err != nil {
			return nil, err
		}
	}

	return files, nil
}

// readProcessFiles appends the files a process holds open.
func (c *PCIDriverRebindController) readProcessFiles(pid int, files []openFile) ([]openFile, error) {
	fdDir := filepath.Join(c.ProcfsPath, strconv.Itoa(pid), "fd")

	fds, err := os.ReadDir(fdDir)
	if err != nil {
		if gone(err) {
			return files, nil
		}

		return nil, err
	}

	for _, fd := range fds {
		file, ok, err := c.readOpenFile(pid, fdDir, fd.Name())
		if err != nil {
			return nil, err
		}

		if ok {
			files = append(files, file)
		}
	}

	return files, nil
}

// readOpenFile reads a file a process holds open on a descriptor, or false when it was closed.
func (c *PCIDriverRebindController) readOpenFile(pid int, fdDir, fd string) (openFile, bool, error) {
	target, err := os.Readlink(filepath.Join(fdDir, fd))
	if err != nil {
		if gone(err) {
			return openFile{}, false, nil
		}

		return openFile{}, false, err
	}

	file := openFile{pid: pid, target: target}

	switch {
	case target == vfioDeviceFD:
		file.vfioSyspath, err = c.vfioDeviceFDSyspath(pid, fd)
	case strings.HasPrefix(target, "/"):
		file.number, file.isDevice, err = c.devices.DeviceNumber(filepath.Join(fdDir, fd))
	}

	switch {
	case err != nil && gone(err):
		// Closed meanwhile: it holds nothing.
		return openFile{}, false, nil
	case err != nil:
		return openFile{}, false, err
	}

	return file, true, nil
}

// gone reports an error reading a process which exited, or a descriptor it closed, meanwhile.
func gone(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, unix.ESRCH)
}

// vfioNodes returns the VFIO device nodes a user opens a device through. Its IOMMU group, or its
// iommufd node, not being readable is an error: the device may be held through it.
func (c *PCIDriverRebindController) vfioNodes(pciID string) ([]string, error) {
	var nodes []string

	group, err := os.Readlink(filepath.Join(c.devicePath(pciID), "iommu_group"))

	switch {
	case err == nil:
		// Without an IOMMU (vfio's no-IOMMU mode), the group node is named apart.
		nodes = append(nodes, "/dev/vfio/"+filepath.Base(group), "/dev/vfio/noiommu-"+filepath.Base(group))
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("error reading IOMMU group: %w", err)
	}

	// With iommufd, a device is opened through its own character device instead of its group.
	cdevs, err := os.ReadDir(filepath.Join(c.devicePath(pciID), "vfio-dev"))

	switch {
	case err == nil:
		for _, cdev := range cdevs {
			nodes = append(nodes, "/dev/vfio/devices/"+cdev.Name())
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("error listing VFIO device nodes: %w", err)
	}

	return nodes, nil
}

// vfioDeviceFD is how a VFIO device file descriptor, obtained from its group, shows in /proc.
const vfioDeviceFD = "anon_inode:[vfio-device]"

// vfioDeviceFDSyspath returns the sysfs path of the device of a VFIO device file descriptor, as its
// fdinfo names it, or an empty string on kernels which do not name it there: such a descriptor is
// taken to hold any device, as unbinding a device a VFIO user still holds blocks until the user
// lets it go.
func (c *PCIDriverRebindController) vfioDeviceFDSyspath(pid int, fd string) (string, error) {
	info, err := os.ReadFile(filepath.Join(c.ProcfsPath, strconv.Itoa(pid), "fdinfo", fd))
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(string(info), "\n") {
		if value, ok := strings.CutPrefix(line, "vfio-device-syspath:"); ok {
			return strings.TrimSpace(value), nil
		}
	}

	return "", nil
}

// deviceSyspath returns the sysfs path of a device as the kernel names it, e.g.
// "/sys/devices/pci0000:e0/0000:e0:01.1/0000:e1:00.0".
func (c *PCIDriverRebindController) deviceSyspath(pciID string) string {
	resolved, err := filepath.EvalSymlinks(c.devicePath(pciID))
	if err != nil {
		resolved = c.devicePath(pciID)
	}

	root := c.SysfsPath
	if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = resolvedRoot
	}

	return "/sys" + strings.TrimPrefix(resolved, root)
}

func (c *PCIDriverRebindController) devicePath(pciID string) string {
	return filepath.Join(c.SysfsPath, "bus", "pci", "devices", pciID)
}

// writeStatus publishes the status of a device, and logs it when it changed.
func (c *PCIDriverRebindController) writeStatus(ctx context.Context, r controller.Runtime, logger *zap.Logger,
	status hardware.PCIDriverRebindStatusSpec,
) error {
	if err := safe.WriterModify(ctx, r, hardware.NewPCIDriverRebindStatus(status.PCIID), func(res *hardware.PCIDriverRebindStatus) error {
		*res.TypedSpec() = status

		return nil
	}); err != nil {
		return fmt.Errorf("error updating PCI rebind status: %w", err)
	}

	if previous, ok := c.reported[status.PCIID]; ok && previous == status {
		return nil
	}

	fields := []zap.Field{
		zap.String("id", status.PCIID),
		zap.String("targetDriver", status.TargetDriver),
		zap.String("driver", status.Driver),
		zap.Stringer("phase", status.Phase),
	}

	switch {
	case onTarget(status.Phase):
		logger.Info("PCI device bound to target driver", fields...)
	case status.Phase == hardware.PCIDriverRebindPhaseMoving:
		logger.Info("PCI device moving to target driver", fields...)
	case status.TargetDriver == "":
		logger.Warn("PCI device is waiting to be handed back to its default driver", append(fields, zap.String("reason", status.Reason))...)
	default:
		logger.Warn("PCI device is not bound to its target driver", append(fields, zap.String("reason", status.Reason))...)
	}

	c.reported[status.PCIID] = status

	return nil
}

func driverName(driver string) string {
	if driver == "" {
		return "no driver"
	}

	return strconv.Quote(driver)
}
