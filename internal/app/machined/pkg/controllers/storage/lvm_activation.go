// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/lvm"
	"github.com/siderolabs/talos/internal/pkg/pci"
	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// LVMActivationController activates discovered LVM volume groups.
//
// Volume groups on disks of a PCI device which a PCIDriverRebindConfig takes from the host (e.g. a
// drive passed through to a virtual machine) are left alone: they belong to the device's new user,
// and an activated volume would hold the disk and keep it from being rebound. Nothing is activated
// before the machine configuration is loaded, as it is what lists those devices.
//
// It fails closed: a physical volume whose PCI device cannot be told yet is retried rather than
// activated, and a volume group is only activated once none of its physical volumes is, or may
// be, on such a device. Activation is per volume group, so one physical volume missed would
// activate all of them.
type LVMActivationController struct {
	V1Alpha1Mode machineruntime.Mode
	LVM          *lvm.LVM
	// SysfsPath is where the host's sysfs is read from to find the PCI devices of a disk. Empty
	// reads /sys.
	SysfsPath string

	// lvm overrides LVM in tests.
	lvm lvmOperations

	seenVolumes    map[string]struct{}
	activatedVGs   map[string]struct{}
	skippedVolumes map[string]struct{}
	heldVGs        map[string]struct{}
}

// lvmOperations is what activation needs from LVM.
type lvmOperations interface {
	PVS(ctx context.Context) ([]lvm.PV, error)
	PVScanAutoActivation(ctx context.Context, devicePath string) (map[string]string, error)
	VGChangeActivate(ctx context.Context, vgName string) error
}

// lvmActivationRetryInterval is how often physical volumes whose device could not be placed yet
// are looked at again, while no other event arrives.
const lvmActivationRetryInterval = 5 * time.Second

// Name implements controller.Controller interface.
func (ctrl *LVMActivationController) Name() string {
	return "storage.LVMActivationController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LVMActivationController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveredVolumeType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			ID:        optional.Some(constants.MetaPartitionLabel),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("udevd"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiskType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LVMActivationController) Outputs() []controller.Output {
	return nil
}

// preconditions wait for udevd, META and the machine configuration.
func (ctrl *LVMActivationController) preconditions(ctx context.Context, r controller.Reader, logger *zap.Logger) (bool, error) {
	for _, ready := range []func(context.Context, controller.Reader, *zap.Logger) (bool, error){
		machineConfigLoaded,
		udevdReady,
		metaReady,
	} {
		if ok, err := ready(ctx, r, logger); err != nil || !ok {
			return false, err
		}
	}

	return true, nil
}

func machineConfigLoaded(ctx context.Context, r controller.Reader, logger *zap.Logger) (bool, error) {
	machineConfig, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to get machine config: %w", err)
	}

	if machineConfig == nil {
		// PCIDriverRebindConfig documents come with it: activating before would take volumes on
		// devices meant to be handed over.
		logger.Debug("waiting for the machine config")

		return false, nil
	}

	return true, nil
}

func udevdReady(ctx context.Context, r controller.Reader, logger *zap.Logger) (bool, error) {
	udevdService, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "udevd")
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to get udevd service: %w", err)
	}

	if udevdService == nil {
		logger.Debug("udevd service not registered yet")

		return false, nil
	}

	if !udevdService.TypedSpec().Running || !udevdService.TypedSpec().Healthy {
		logger.Debug("waiting for udevd service to be running and healthy")

		return false, nil
	}

	return true, nil
}

func metaReady(ctx context.Context, r controller.Reader, logger *zap.Logger) (bool, error) {
	meta, err := safe.ReaderGetByID[*block.VolumeStatus](ctx, r, constants.MetaPartitionLabel)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to get meta partition info: %w", err)
	}

	if meta == nil {
		logger.Debug("meta partition not registered yet")

		return false, nil
	}

	if meta.TypedSpec().Phase != block.VolumePhaseReady {
		logger.Debug("meta partition not ready yet")

		return false, nil
	}

	return true, nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *LVMActivationController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.seenVolumes == nil {
		ctrl.seenVolumes = map[string]struct{}{}
	}

	if ctrl.activatedVGs == nil {
		ctrl.activatedVGs = map[string]struct{}{}
	}

	if ctrl.skippedVolumes == nil {
		ctrl.skippedVolumes = map[string]struct{}{}
	}

	if ctrl.heldVGs == nil {
		ctrl.heldVGs = map[string]struct{}{}
	}

	if ctrl.SysfsPath == "" {
		ctrl.SysfsPath = "/sys"
	}

	if ctrl.V1Alpha1Mode.IsAgent() {
		// in agent mode, we don't want to activate LVMs
		return nil
	}

	var retry <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-retry:
			retry = nil
		}

		ok, err := ctrl.preconditions(ctx, r, logger)
		if err != nil {
			return err
		}

		if !ok {
			continue
		}

		discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list discovered volumes: %w", err)
		}

		taken, err := ctrl.takenAddresses(ctx, r)
		if err != nil {
			return err
		}

		pending, err := ctrl.activate(ctx, logger, discoveredVolumes, taken)
		if pending && retry == nil {
			retry = time.After(lvmActivationRetryInterval)
		}

		if err != nil {
			return err
		}
	}
}

// activate activates the volume groups of the discovered physical volumes which are not, and may
// not be, on a device taken from the host. It reports whether any is left to look at again.
//
//nolint:gocyclo
func (ctrl *LVMActivationController) activate(ctx context.Context, logger *zap.Logger,
	discoveredVolumes safe.List[*block.DiscoveredVolume], taken map[string]struct{},
) (bool, error) {
	var (
		multiErr error
		pending  bool
		// pvs is listed once per pass, when first needed.
		pvs []lvm.PV
	)

	for dv := range discoveredVolumes.All() {
		if dv.TypedSpec().Name != "lvm2-pv" {
			// Only activate pre-existing LVM volumes, not ones just formatted.
			ctrl.seenVolumes[dv.Metadata().ID()] = struct{}{}

			continue
		}

		if _, ok := ctrl.seenVolumes[dv.Metadata().ID()]; ok {
			continue
		}

		// Not marked as seen: should the configuration be removed, the volume is activated.
		switch placement, address := placeDevice(ctrl.SysfsPath, filepath.Base(dv.TypedSpec().DevPath), taken); placement {
		case placementTaken:
			if _, logged := ctrl.skippedVolumes[dv.Metadata().ID()]; !logged {
				logger.Info("skipping LVM volume activation on a device taken from the host",
					zap.String("device", dv.TypedSpec().DevPath), zap.String("pciID", address))

				ctrl.skippedVolumes[dv.Metadata().ID()] = struct{}{}
			}

			continue
		case placementUnknown:
			logger.Debug("waiting to find the PCI device of an LVM volume", zap.String("device", dv.TypedSpec().DevPath))

			pending = true

			continue
		case placementHost:
		}

		delete(ctrl.skippedVolumes, dv.Metadata().ID())

		// The volume group is checked before pvscan: with --vgonline, pvscan records the volume group
		// as online and reports it as ready only once, so a volume group held back after it would
		// never be reported again.
		if held, err := ctrl.holdVolumeGroup(ctx, logger, dv.TypedSpec().DevPath, taken, &pvs); err != nil || held != placementHost {
			if err != nil {
				multiErr = multierror.Append(multiErr, err)
			}

			pending = pending || held == placementUnknown

			continue
		}

		logger.Debug("checking device for LVM volume activation", zap.String("device", dv.TypedSpec().DevPath))

		vgName, err := ctrl.checkVGNeedsActivation(ctx, dv.TypedSpec().DevPath)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)

			continue
		}

		if vgName == "" {
			continue
		}

		if _, ok := ctrl.activatedVGs[vgName]; ok {
			continue
		}

		delete(ctrl.heldVGs, vgName)

		logger.Info("activating LVM volume", zap.String("name", vgName),
			zap.String("device", dv.TypedSpec().DevPath), zap.Int("takenDevices", len(taken)))

		if err = ctrl.ops().VGChangeActivate(ctx, vgName); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("failed to activate LVM volume %s: %w", vgName, err))
		} else {
			ctrl.activatedVGs[vgName] = struct{}{}
		}
	}

	return pending, multiErr
}

// takenAddresses returns the addresses of the PCI devices a PCIDriverRebindConfig takes from the host.
//
// They are read from the machine configuration itself, the same one preconditions waited for, not
// from the PCIDriverRebindConfig resources: those are derived from it by another controller, and
// may not exist yet on the first pass after it is loaded, when nothing would be seen as taken.
func (ctrl *LVMActivationController) takenAddresses(ctx context.Context, r controller.Reader) (map[string]struct{}, error) {
	cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil {
		return nil, fmt.Errorf("failed to get machine config: %w", err)
	}

	return configTakenAddresses(cfg.Config()), nil
}

// configTakenAddresses returns the addresses of the PCI devices the PCIDriverRebindConfig
// documents of a machine configuration take from the host.
func configTakenAddresses(cfg talosconfig.Config) map[string]struct{} {
	rebinds := cfg.PCIDriverRebindConfig().PCIDriverRebindConfigs()
	taken := make(map[string]struct{}, len(rebinds))

	for _, rebind := range rebinds {
		taken[rebind.PCIID()] = struct{}{}
	}

	return taken
}

// placement says where a block device is, relative to the devices taken from the host.
type placement int

const (
	// placementHost is a device on no PCI device taken from the host.
	placementHost placement = iota
	// placementTaken is a device on a PCI device taken from the host.
	placementTaken
	// placementUnknown is a device whose PCI devices cannot be told yet.
	placementUnknown
)

// placeDevice places a block device by its sysfs name, and returns the address of the PCI device
// taken from the host it is on, if any.
//
// It reads sysfs directly rather than the Disk resources, which may not exist yet for a disk whose
// physical volume has already been discovered.
func placeDevice(sysfsPath, name string, taken map[string]struct{}) (placement, string) {
	if len(taken) == 0 {
		return placementHost, ""
	}

	addresses, known := pci.BlockDeviceAddresses(sysfsPath, name)

	for _, address := range addresses {
		if _, ok := taken[address]; ok {
			return placementTaken, address
		}
	}

	if !known {
		return placementUnknown, ""
	}

	return placementHost, ""
}

// placeVolumeGroup places a volume group by its physical volumes: taken from the host if any of
// them is, else unknown if any of them cannot be placed yet. It returns the device which decided.
func placeVolumeGroup(sysfsPath, vgName string, pvs []lvm.PV, taken map[string]struct{}) (placement, string) {
	result, decidedBy := placementHost, ""

	for _, pv := range pvs {
		if pv.VGName != vgName {
			continue
		}

		// A physical volume whose device is gone (e.g. a drive which did not enumerate on this
		// boot) cannot be placed: it may well be on a device taken from the host.
		if pv.Missing != "" || pv.Device == "" || strings.HasPrefix(pv.Device, "[unknown") {
			result, decidedBy = placementUnknown, pv.Device

			continue
		}

		device := pv.Device
		if resolved, err := filepath.EvalSymlinks(device); err == nil {
			device = resolved
		}

		switch placement, _ := placeDevice(sysfsPath, filepath.Base(device), taken); placement {
		case placementTaken:
			return placementTaken, pv.Device
		case placementUnknown:
			result, decidedBy = placementUnknown, pv.Device
		case placementHost:
		}
	}

	return result, decidedBy
}

// ops returns the LVM operations activation runs.
func (ctrl *LVMActivationController) ops() lvmOperations {
	if ctrl.lvm != nil {
		return ctrl.lvm
	}

	return ctrl.LVM
}

// holdVolumeGroup places the volume group of a physical volume, listed with a read-only pvs, and
// logs it the first time it is held back. A physical volume in no volume group, or a host without
// devices taken from it, is placed on the host.
func (ctrl *LVMActivationController) holdVolumeGroup(ctx context.Context, logger *zap.Logger, devPath string,
	taken map[string]struct{}, pvs *[]lvm.PV,
) (placement, error) {
	if len(taken) == 0 {
		return placementHost, nil
	}

	if *pvs == nil {
		listed, err := ctrl.ops().PVS(ctx)
		if err != nil {
			return placementUnknown, fmt.Errorf("failed to list LVM physical volumes: %w", err)
		}

		*pvs = listed
	}

	vgName := volumeGroupOf(*pvs, devPath)
	if vgName == "" {
		return placementHost, nil
	}

	placement, device := placeVolumeGroup(ctrl.SysfsPath, vgName, *pvs, taken)
	if placement == placementHost {
		delete(ctrl.heldVGs, vgName)

		return placementHost, nil
	}

	if _, logged := ctrl.heldVGs[vgName]; !logged {
		logger.Warn("not activating an LVM volume group with a physical volume on a device which is, or may be, taken from the host",
			zap.String("name", vgName), zap.String("device", device))

		ctrl.heldVGs[vgName] = struct{}{}
	}

	return placement, nil
}

// volumeGroupOf returns the volume group of the physical volume on the given device, or an empty
// string when it is in none.
func volumeGroupOf(pvs []lvm.PV, devPath string) string {
	device := devPath
	if resolved, err := filepath.EvalSymlinks(devPath); err == nil {
		device = resolved
	}

	for _, pv := range pvs {
		pvDevice := pv.Device
		if resolved, err := filepath.EvalSymlinks(pvDevice); err == nil {
			pvDevice = resolved
		}

		if pvDevice == device {
			return pv.VGName
		}
	}

	return ""
}

// checkVGNeedsActivation returns VG name if auto-activation is needed.
func (ctrl *LVMActivationController) checkVGNeedsActivation(ctx context.Context, devicePath string) (string, error) {
	udev, err := ctrl.ops().PVScanAutoActivation(ctx, devicePath)
	if err != nil {
		return "", fmt.Errorf("failed to check if LVM volume backed by device %s needs activation: %w", devicePath, err)
	}

	return udev[lvm.UdevKeyVGNameComplete], nil
}
