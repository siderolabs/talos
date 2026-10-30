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

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/kobject"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/trigger"
	runtimectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/pci"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// pciDriverRebindRetryInterval is how often devices which could not be brought to their target
// driver, or released, are retried while no other event arrives.
const pciDriverRebindRetryInterval = 10 * time.Second

// PCIDriverRebindController binds PCI devices to a specific driver and unbinds them from the host driver.
//
// A device already bound to its target driver is left alone. No device is ever unbound while it is in
// use: while a VFIO user (e.g. a virtual machine) holds it, as unbinding makes the user release the
// device, hot-unplugging it from a running guest; or while its disks are mounted or held by another
// block device (e.g. an activated LVM volume group), which would fail every I/O on them. Devices are
// reconciled again whenever a PCI device is added or bound, so a device which re-enumerates is
// brought back to its target driver.
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

// blockLayout is where the host's block devices are, read from sysfs.
type blockLayout struct {
	// disks are the block devices on each PCI device, e.g. the namespaces of an NVMe controller,
	// multipath namespace heads reached through it, and device-mapper devices built on them.
	disks map[string][]string
	// unplaced are the block devices which cannot be placed yet, e.g. a multipath head with no path
	// or a device-mapper device with no slave yet.
	unplaced []string
}

// blockLayout reads where the host's block devices are from sysfs alone: Disk resources may not
// exist yet for a block device the host already uses.
func (c *PCIDriverRebindController) blockLayout() (blockLayout, error) {
	layout := blockLayout{disks: map[string][]string{}}

	entries, err := os.ReadDir(filepath.Join(c.SysfsPath, "class", "block"))
	if errors.Is(err, fs.ErrNotExist) {
		return layout, nil
	}

	if err != nil {
		return layout, fmt.Errorf("error listing block devices: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()

		// Partitions are checked with their disk.
		if _, err := os.Stat(filepath.Join(c.SysfsPath, "class", "block", name, "partition")); err == nil {
			continue
		}

		addresses, ok := pci.BlockDeviceAddresses(c.SysfsPath, name)
		if !ok {
			layout.unplaced = append(layout.unplaced, name)

			continue
		}

		for _, address := range addresses {
			layout.disks[address] = append(layout.disks[address], name)
		}
	}

	return layout, nil
}

// reconcile brings every configured device to its target driver, and hands back the devices
// whose configuration was removed. It reports whether any device is left to retry.
func (c *PCIDriverRebindController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (bool, error) {
	configs, err := safe.ReaderListAll[*hardware.PCIDriverRebindConfig](ctx, r)
	if err != nil {
		return false, fmt.Errorf("error listing all PCI rebind configs: %w", err)
	}

	blocks, err := c.blockLayout()
	if err != nil {
		return false, err
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

	configured, pending, err := c.bindConfigured(ctx, r, logger, configs, blocks, previous)
	if err != nil {
		return false, err
	}

	released, err := c.releaseRemoved(ctx, r, logger, configured, blocks, previous)
	if err != nil {
		return false, err
	}

	pending = pending || released

	return pending, safe.CleanupOutputs[*hardware.PCIDriverRebindStatus](ctx, r)
}

// bindConfigured brings every configured device to its target driver. It returns the configured
// devices, and whether any of them is left to retry.
func (c *PCIDriverRebindController) bindConfigured(ctx context.Context, r controller.Runtime, logger *zap.Logger,
	configs safe.List[*hardware.PCIDriverRebindConfig], blocks blockLayout, previous map[string]hardware.PCIDriverRebindStatusSpec,
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

		status, err := c.bind(pciID, target, blocks, previous[pciID], record)
		if err != nil {
			return nil, false, err
		}

		pending = pending || !onTarget(status.Phase)

		if err := c.writeStatus(ctx, r, logger, status); err != nil {
			return nil, false, err
		}
	}

	return configured, pending, nil
}

// releaseRemoved hands back the devices whose configuration was removed. It reports whether any of
// them is left to retry, as it cannot be handed back yet.
func (c *PCIDriverRebindController) releaseRemoved(ctx context.Context, r controller.Runtime, logger *zap.Logger,
	configured map[string]struct{}, blocks blockLayout, previous map[string]hardware.PCIDriverRebindStatusSpec,
) (bool, error) {
	var pending bool

	for pciID, status := range previous {
		if _, ok := configured[pciID]; ok {
			continue
		}

		status, released := c.release(status, blocks)
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
func (c *PCIDriverRebindController) bind(pciID, target string, blocks blockLayout, previous hardware.PCIDriverRebindStatusSpec,
	record func(hardware.PCIDriverRebindStatusSpec) error,
) (hardware.PCIDriverRebindStatusSpec, error) {
	status := hardware.PCIDriverRebindStatusSpec{PCIID: pciID, TargetDriver: target}

	driver, err := c.boundDriver(pciID)
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

	switch {
	case errors.Is(err, fs.ErrNotExist):
		status.Reason = err.Error()
		status.Phase = hardware.PCIDriverRebindPhasePending

		// A device the controller moved stays its to hand back, should it come back on the target driver.
		if moved {
			status.Phase = hardware.PCIDriverRebindPhaseUnbound
		}

		return status, nil
	case err != nil:
		return stay(err.Error())
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

	if reason := c.inUse(pciID, driver, blocks); reason != "" {
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
	}

	if err := c.setOverride(pciID, target); err != nil {
		return stay(err.Error())
	}

	if driver != "" {
		if err := c.unbind(pciID); err != nil {
			return stay(err.Error())
		}
	}

	// From here on, the device has left the driver it had: it is the controller's to hand back.
	status.Phase = hardware.PCIDriverRebindPhaseUnbound

	unbound := func(reason string) (hardware.PCIDriverRebindStatusSpec, error) {
		status.Reason = reason

		return status, nil
	}

	if err := c.probe(pciID); err != nil {
		return unbound(err.Error())
	}

	if driver, err = c.boundDriver(pciID); err != nil {
		return unbound(err.Error())
	}

	status.Driver = driver

	if driver != target {
		status.Reason = fmt.Sprintf("device is bound to %s, ensure the %s driver is available (its kernel module is loaded)",
			driverName(driver), target)

		return status, nil
	}

	status.Phase = hardware.PCIDriverRebindPhaseBound

	return status, nil
}

// release hands a device whose configuration was removed back to its default driver, if the
// controller moved it and it is no longer in use. A device the controller never moved keeps its
// driver: only the driver override the controller left is cleared.
func (c *PCIDriverRebindController) release(previous hardware.PCIDriverRebindStatusSpec, blocks blockLayout) (hardware.PCIDriverRebindStatusSpec, bool) {
	pciID := previous.PCIID
	status := hardware.PCIDriverRebindStatusSpec{PCIID: pciID, Phase: previous.Phase}

	driver, err := c.boundDriver(pciID)
	if errors.Is(err, fs.ErrNotExist) {
		// The device is gone: there is nothing left to hand back.
		return status, true
	}

	if err != nil {
		status.Reason = err.Error()

		return status, false
	}

	status.Driver = driver

	if !wasMoved(previous, driver) {
		// Clearing the override does not unbind the device: it only stops a later probe from binding
		// it to the target the configuration named.
		if c.overrideIsSet(pciID) {
			if err := c.setOverride(pciID, "\n"); err != nil {
				status.Reason = err.Error()

				return status, false
			}
		}

		return status, true
	}

	if c.backOnDefault(pciID, driver, previous.Driver) {
		return status, true
	}

	status.Phase = hardware.PCIDriverRebindPhaseReleasing

	if reason := c.inUse(pciID, driver, blocks); reason != "" {
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
// it may be.
func (c *PCIDriverRebindController) inUse(pciID, driver string, blocks blockLayout) string {
	if reason := c.heldByVFIOUser(pciID); reason != "" {
		return reason
	}

	// A block device which cannot be placed yet may be on this device, unless the device has no
	// driver to provide one, or is on a VFIO driver.
	if len(blocks.unplaced) > 0 && driver != "" && !strings.Contains(driver, "vfio") && c.isStorage(pciID) {
		return fmt.Sprintf("held: cannot tell yet whether %s is on this device", blocks.unplaced[0])
	}

	return c.heldByBlockUser(blocks.disks[pciID])
}

// isStorage reports whether a device is a mass storage controller, or may be one as its class
// cannot be read.
func (c *PCIDriverRebindController) isStorage(pciID string) bool {
	class, err := os.ReadFile(filepath.Join(c.devicePath(pciID), "class"))
	if err != nil {
		return true
	}

	return strings.HasPrefix(strings.TrimSpace(string(class)), "0x01")
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
	if err := c.writeSysfs(filepath.Join(c.devicePath(pciID), "driver", "unbind"), []byte(pciID)); err != nil &&
		!errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("error unbinding device with id: %s, %w", pciID, err)
	}

	return nil
}

// probe asks the kernel to bind a device to the driver its override, or its default, names.
func (c *PCIDriverRebindController) probe(pciID string) error {
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
func (c *PCIDriverRebindController) overrideIsSet(pciID string) bool {
	contents, err := os.ReadFile(filepath.Join(c.devicePath(pciID), "driver_override"))
	if err != nil {
		return false
	}

	override := string(bytes.TrimSpace(contents))

	return override != "" && override != "(null)"
}

// backOnDefault reports whether a device the controller moved came back (e.g. re-enumerated) on
// another driver than it last had, and without the override: it is on its default driver already,
// with nothing left to hand back.
func (c *PCIDriverRebindController) backOnDefault(pciID, driver, previous string) bool {
	return driver != "" && driver != previous && c.overrideCleared(pciID)
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
// iommufd node, or through a device file descriptor obtained from its group, which a process may
// keep after closing the group.
func (c *PCIDriverRebindController) vfioUser(pciID string) (int, error) {
	nodes := c.vfioNodes(pciID)
	if len(nodes) == 0 {
		return 0, nil
	}

	syspath := c.deviceSyspath(pciID)

	procs, err := os.ReadDir(c.ProcfsPath)
	if err != nil {
		return 0, err
	}

	for _, proc := range procs {
		pid, err := strconv.Atoi(proc.Name())
		if err != nil {
			continue
		}

		if c.holdsAny(proc.Name(), nodes, syspath) {
			return pid, nil
		}
	}

	return 0, nil
}

// vfioNodes returns the VFIO device nodes a user opens a device through.
func (c *PCIDriverRebindController) vfioNodes(pciID string) []string {
	var nodes []string

	if group, err := os.Readlink(filepath.Join(c.devicePath(pciID), "iommu_group")); err == nil {
		nodes = append(nodes, "/dev/vfio/"+filepath.Base(group))
	}

	// With iommufd, a device is opened through its own character device instead of its group.
	if cdevs, err := os.ReadDir(filepath.Join(c.devicePath(pciID), "vfio-dev")); err == nil {
		for _, cdev := range cdevs {
			nodes = append(nodes, "/dev/vfio/devices/"+cdev.Name())
		}
	}

	return nodes
}

// vfioDeviceFD is how a VFIO device file descriptor, obtained from its group, shows in /proc.
const vfioDeviceFD = "anon_inode:[vfio-device]"

// holdsAny reports whether a process has any of the given device nodes open, or a VFIO device file
// descriptor for the device at the given sysfs path.
func (c *PCIDriverRebindController) holdsAny(pid string, nodes []string, syspath string) bool {
	fds, err := os.ReadDir(filepath.Join(c.ProcfsPath, pid, "fd"))
	if err != nil {
		// The process exited, or its descriptors are not readable.
		return false
	}

	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join(c.ProcfsPath, pid, "fd", fd.Name()))
		if err != nil {
			continue
		}

		if slices.Contains(nodes, target) {
			return true
		}

		if target == vfioDeviceFD && c.vfioDeviceFDHolds(pid, fd.Name(), syspath) {
			return true
		}
	}

	return false
}

// vfioDeviceFDHolds reports whether a VFIO device file descriptor is for the device at the given
// sysfs path, as its fdinfo names it. Kernels which do not name it there are assumed to hold the
// device: unbinding a device a VFIO user still holds blocks until the user lets it go.
func (c *PCIDriverRebindController) vfioDeviceFDHolds(pid, fd, syspath string) bool {
	info, err := os.ReadFile(filepath.Join(c.ProcfsPath, pid, "fdinfo", fd))
	if err != nil {
		// The descriptor was closed in the meantime.
		return false
	}

	for line := range strings.SplitSeq(string(info), "\n") {
		if value, ok := strings.CutPrefix(line, "vfio-device-syspath:"); ok {
			return strings.TrimSpace(value) == syspath
		}
	}

	return true
}

// deviceSyspath returns the sysfs path of a device as the kernel names it, e.g.
// "/sys/devices/pci0000:e0/0000:e0:01.1/0000:e1:00.0".
func (c *PCIDriverRebindController) deviceSyspath(pciID string) string {
	resolved, err := filepath.EvalSymlinks(c.devicePath(pciID))
	if err != nil {
		resolved = c.devicePath(pciID)
	}

	return "/sys" + strings.TrimPrefix(resolved, c.SysfsPath)
}

// heldByBlockUser returns why a device whose disks are in use on the host must not be unbound, or
// an empty string when it may be.
func (c *PCIDriverRebindController) heldByBlockUser(disks []string) string {
	if len(disks) == 0 {
		return ""
	}

	mounted, err := c.mountedDevices()
	if err != nil {
		return fmt.Sprintf("held: cannot determine whether the device's disks are in use: %s", err)
	}

	for _, disk := range disks {
		for _, blockDevice := range c.blockDevices(disk) {
			holders, err := os.ReadDir(filepath.Join(blockDevice, "holders"))
			if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Sprintf("held: cannot determine whether the device's disks are in use: %s", err)
			}

			if len(holders) > 0 {
				return fmt.Sprintf("held: %s is in use by %s, release it before changing the driver",
					filepath.Base(blockDevice), holders[0].Name())
			}

			dev, err := os.ReadFile(filepath.Join(blockDevice, "dev"))
			if err == nil {
				if _, ok := mounted[strings.TrimSpace(string(dev))]; ok {
					return fmt.Sprintf("held: %s is mounted, unmount it before changing the driver", filepath.Base(blockDevice))
				}
			}
		}
	}

	return ""
}

// blockDevices returns the sysfs directories of a disk and of its partitions.
func (c *PCIDriverRebindController) blockDevices(disk string) []string {
	diskPath := filepath.Join(c.SysfsPath, "class", "block", disk)
	devices := []string{diskPath}

	entries, err := os.ReadDir(diskPath)
	if err != nil {
		return devices
	}

	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join(diskPath, entry.Name(), "partition")); err == nil {
			devices = append(devices, filepath.Join(diskPath, entry.Name()))
		}
	}

	return devices
}

// mountedDevices returns the device numbers ("major:minor") of the mounted block devices.
func (c *PCIDriverRebindController) mountedDevices() (map[string]struct{}, error) {
	contents, err := os.ReadFile(filepath.Join(c.ProcfsPath, "self", "mountinfo"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	mounted := map[string]struct{}{}

	for line := range strings.Lines(string(contents)) {
		// The third field is the device number of the mounted filesystem.
		if fields := strings.Fields(line); len(fields) > 2 {
			mounted[fields[2]] = struct{}{}
		}
	}

	return mounted, nil
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
