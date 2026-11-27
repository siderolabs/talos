// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/go-blockdevice/v2/devicemapper"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// DeviceMapperControl is the device-mapper subset used by DeviceMapperCloseController.
type DeviceMapperControl interface {
	ListDevices() ([]devicemapper.DeviceInfo, error)
	RemoveDevice(id devicemapper.DeviceID) error
	Close() error
}

// DeviceMapperCloseController removes the device-mapper devices stacked on top of a volume being torn down
// (e.g. LVM logical volumes on an encrypted raw volume), so that the volume can be closed.
//
// It holds a finalizer on every volume backed by a block device, and releases it once nothing is stacked on the volume.
type DeviceMapperCloseController struct {
	V1Alpha1Mode machineruntime.Mode

	// NewControl opens the device-mapper control, defaults to devicemapper.NewControl.
	NewControl func() (DeviceMapperControl, error)

	// RetryInterval defaults to 3 seconds.
	RetryInterval time.Duration

	lastBlocked map[string]string
}

// Name implements controller.Controller interface.
func (ctrl *DeviceMapperCloseController) Name() string {
	return "block.DeviceMapperCloseController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DeviceMapperCloseController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DeviceType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DeviceMapperCloseController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *DeviceMapperCloseController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// in container mode, there are no volumes backed by block devices
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	if ctrl.NewControl == nil {
		ctrl.NewControl = func() (DeviceMapperControl, error) { return devicemapper.NewControl() }
	}

	if ctrl.RetryInterval == 0 {
		ctrl.RetryInterval = 3 * time.Second
	}

	ctrl.lastBlocked = map[string]string{}

	retryTicker := time.NewTicker(ctrl.RetryInterval)
	defer retryTicker.Stop()

	shouldRetry := false

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-retryTicker.C:
			if !shouldRetry {
				continue
			}
		}

		var err error

		if shouldRetry, err = ctrl.reconcile(ctx, r, logger); err != nil {
			return err
		}
	}
}

// reconcile returns true if some volume is still held and the reconcile should be retried.
func (ctrl *DeviceMapperCloseController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (bool, error) {
	volumeStatuses, err := safe.ReaderListAll[*block.VolumeStatus](ctx, r)
	if err != nil {
		return false, fmt.Errorf("error listing volume statuses: %w", err)
	}

	devices, err := safe.ReaderListAll[*block.Device](ctx, r)
	if err != nil {
		return false, fmt.Errorf("error listing devices: %w", err)
	}

	graph := newDeviceGraph(devices, volumeStatuses)
	shouldRetry := false

	for volumeStatus := range volumeStatuses.All() {
		switch volumeStatus.TypedSpec().Type {
		case block.VolumeTypeDisk, block.VolumeTypePartition:
		case block.VolumeTypeTmpfs, block.VolumeTypeDirectory, block.VolumeTypeSymlink, block.VolumeTypeOverlay, block.VolumeTypeExternal:
			continue
		}

		hasFinalizer := volumeStatus.Metadata().Finalizers().Has(ctrl.Name())

		if volumeStatus.Metadata().Phase() == resource.PhaseRunning {
			if !hasFinalizer {
				if err = r.AddFinalizer(ctx, volumeStatus.Metadata(), ctrl.Name()); err != nil {
					return false, fmt.Errorf("error adding finalizer to volume status %q: %w", volumeStatus.Metadata().ID(), err)
				}
			}

			continue
		}

		if !hasFinalizer {
			continue
		}

		volumeID := volumeStatus.Metadata().ID()

		if err = ctrl.closeHolders(logger, graph, volumeStatus); err != nil {
			if ctrl.lastBlocked[volumeID] != err.Error() {
				logger.Info("waiting for the devices stacked on the volume to be closed", zap.String("volume", volumeID), zap.Error(err))

				ctrl.lastBlocked[volumeID] = err.Error()
			}

			shouldRetry = true

			continue
		}

		delete(ctrl.lastBlocked, volumeID)

		if err = r.RemoveFinalizer(ctx, volumeStatus.Metadata(), ctrl.Name()); err != nil {
			return false, fmt.Errorf("error removing finalizer from volume status %q: %w", volumeID, err)
		}
	}

	return shouldRetry, nil
}

// closeHolders removes all device-mapper devices stacked on the device of the volume, topmost first.
func (ctrl *DeviceMapperCloseController) closeHolders(logger *zap.Logger, graph *deviceGraph, volumeStatus *block.VolumeStatus) error {
	// MountLocation is the device the volume is consumed through, e.g. the opened device of an encrypted volume
	// (which the volume manager closes itself), so only the devices stacked on top of it are of interest
	root := devPathToID(volumeStatus.TypedSpec().MountLocation)
	if root == "" {
		return nil
	}

	order, err := graph.removalOrder(root)
	if err != nil || len(order) == 0 {
		return err
	}

	control, err := ctrl.NewControl()
	if err != nil {
		return err
	}

	defer control.Close() //nolint:errcheck

	dmDevices, err := control.ListDevices()
	if err != nil {
		return err
	}

	namesByDevNo := make(map[uint64]string, len(dmDevices))

	for _, dmDevice := range dmDevices {
		namesByDevNo[dmDevice.DevNo] = dmDevice.Name
	}

	for _, id := range order {
		spec := graph.devices[id].TypedSpec()

		name, ok := namesByDevNo[unix.Mkdev(uint32(spec.Major), uint32(spec.Minor))]
		if !ok {
			// gone already
			continue
		}

		logger.Info("removing device-mapper device",
			zap.String("volume", volumeStatus.Metadata().ID()),
			zap.String("device", id),
			zap.String("name", name),
		)

		if err = control.RemoveDevice(devicemapper.ByName(name)); err != nil && !errors.Is(err, unix.ENXIO) {
			return err
		}
	}

	return nil
}

// deviceGraph tracks which block devices are stacked on top of each other.
type deviceGraph struct {
	devices map[string]*block.Device
	// holders maps a device ID to the IDs of the devices stacked on top of it.
	holders map[string][]string
	// usedBy maps a device ID to the ID of a volume which is not closed yet and uses it.
	usedBy map[string]string
}

func newDeviceGraph(devices safe.List[*block.Device], volumeStatuses safe.List[*block.VolumeStatus]) *deviceGraph {
	graph := &deviceGraph{
		devices: map[string]*block.Device{},
		holders: map[string][]string{},
		usedBy:  map[string]string{},
	}

	for device := range devices.All() {
		id := device.Metadata().ID()
		graph.devices[id] = device

		for _, secondary := range device.TypedSpec().Secondaries {
			graph.holders[secondary] = append(graph.holders[secondary], id)
		}

		// a device-mapper partition map is reported as a partition of the device it maps,
		// but unlike a kernel partition it holds that device open
		if device.TypedSpec().Type == block.DeviceTypePartition && isDeviceMapper(id) {
			graph.holders[device.TypedSpec().Parent] = append(graph.holders[device.TypedSpec().Parent], id)
		}
	}

	for volumeStatus := range volumeStatuses.All() {
		if volumeStatus.TypedSpec().Phase == block.VolumePhaseClosed {
			continue
		}

		for _, location := range []string{volumeStatus.TypedSpec().Location, volumeStatus.TypedSpec().MountLocation} {
			if id := devPathToID(location); id != "" {
				graph.usedBy[id] = volumeStatus.Metadata().ID()
			}
		}
	}

	return graph
}

// removalOrder returns the device-mapper devices stacked on the root device, topmost first.
//
// It fails if anything stacked on the root is used by another volume, or is not a device-mapper device.
func (graph *deviceGraph) removalOrder(root string) ([]string, error) {
	var (
		order []string
		visit func(id string) error
	)

	visited := map[string]struct{}{}

	visit = func(id string) error {
		for _, holder := range graph.holders[id] {
			if _, ok := visited[holder]; ok {
				continue
			}

			visited[holder] = struct{}{}

			if volumeID, ok := graph.usedBy[holder]; ok {
				return fmt.Errorf("device %q is used by volume %q", holder, volumeID)
			}

			if !isDeviceMapper(holder) {
				return fmt.Errorf("device %q is not a device-mapper device", holder)
			}

			if err := visit(holder); err != nil {
				return err
			}

			order = append(order, holder)
		}

		return nil
	}

	return order, visit(root)
}

func isDeviceMapper(id string) bool {
	return strings.HasPrefix(id, "dm-")
}

// devPathToID returns the ID of the block device at the path, or "" if the path is not a block device.
func devPathToID(devPath string) string {
	if !strings.HasPrefix(devPath, "/dev/") {
		return ""
	}

	return filepath.Base(devPath)
}
