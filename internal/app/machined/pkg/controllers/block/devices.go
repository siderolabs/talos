// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/inotify"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/kobject"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/sysblock"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// DevicesController provides a view of available block devices with information about pending updates.
type DevicesController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *DevicesController) Name() string {
	return "block.DevicesController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DevicesController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *DevicesController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.DeviceType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *DevicesController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// in container mode, no devices
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	// start the watcher first
	watcher, err := kobject.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create kobject watcher: %w", err)
	}

	defer watcher.Close() //nolint:errcheck

	watchCh := watcher.Run(logger)

	// start the inotify watcher
	inotifyWatcher, err := inotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create inotify watcher: %w", err)
	}

	defer inotifyWatcher.Close() //nolint:errcheck

	inotifyCh, inotifyErrCh := inotifyWatcher.Run()

	// reconcile the initial list of devices while the watcher is running
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	if err = ctrl.resync(ctx, r, logger, inotifyWatcher); err != nil {
		return fmt.Errorf("failed to resync: %w", err)
	}

	for {
		select {
		case ev := <-watchCh:
			if ev.Subsystem != "block" {
				continue
			}

			ev.DevicePath = filepath.Join("/sys", ev.DevicePath)

			if err = ctrl.processEvent(ctx, r, logger, inotifyWatcher, ev); err != nil {
				return err
			}
		case err = <-inotifyErrCh:
			return fmt.Errorf("inotify watcher failed: %w", err)
		case updatedPath := <-inotifyCh:
			id := filepath.Base(updatedPath)

			if err = ctrl.bumpGeneration(ctx, r, logger, id); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (ctrl *DevicesController) bumpGeneration(ctx context.Context, r controller.Runtime, logger *zap.Logger, id string) error {
	_, err := safe.ReaderGetByID[*block.Device](ctx, r, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			// skip it
			return nil
		}

		return err
	}

	logger.Debug("bumping generation for device, inotify update", zap.String("id", id))

	return safe.WriterModify(ctx, r, block.NewDevice(block.NamespaceName, id), func(dev *block.Device) error {
		dev.TypedSpec().Generation++

		return nil
	})
}

func (ctrl *DevicesController) resync(ctx context.Context, r controller.Runtime, logger *zap.Logger, inotifyWatcher *inotify.Watcher) error {
	events, err := sysblock.Walk("/sys/block")
	if err != nil {
		return fmt.Errorf("failed to walk /sys/block: %w", err)
	}

	touchedIDs := make(map[string]struct{}, len(events))

	for _, ev := range events {
		if err = ctrl.processEvent(ctx, r, logger, inotifyWatcher, ev); err != nil {
			return err
		}

		touchedIDs[ev.Values["DEVNAME"]] = struct{}{}
	}

	// remove devices that were not touched
	devices, err := safe.ReaderListAll[*block.Device](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list devices: %w", err)
	}

	for dev := range devices.All() {
		if _, ok := touchedIDs[dev.Metadata().ID()]; ok {
			continue
		}

		if err = r.Destroy(ctx, dev.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to remove device: %w", err)
		}
	}

	return nil
}

//nolint:gocyclo
func (ctrl *DevicesController) processEvent(ctx context.Context, r controller.Runtime, logger *zap.Logger, inotifyWatcher *inotify.Watcher, ev *kobject.Event) error {
	logger = logger.With(
		zap.String("action", string(ev.Action)),
		zap.String("path", ev.DevicePath),
		zap.String("id", ev.Values["DEVNAME"]),
	)

	logger.Debug("processing event")

	id := ev.Values["DEVNAME"]
	devPath := filepath.Join("/dev", id)

	// re-stat the sysfs entry to make sure we are not out of sync with events
	_, reStatErr := os.Stat(ev.DevicePath)

	switch ev.Action {
	case kobject.ActionAdd, kobject.ActionBind, kobject.ActionOnline, kobject.ActionChange, kobject.ActionMove, kobject.ActionUnbind, kobject.ActionOffline:
		if reStatErr != nil {
			logger.Debug("skipped, as device path doesn't exist")

			return nil //nolint:nilerr // entry doesn't exist now, so skip the event
		}

		if err := safe.WriterModify(ctx, r, block.NewDevice(block.NamespaceName, id), func(dev *block.Device) error {
			dev.TypedSpec().Type = ev.Values["DEVTYPE"]
			dev.TypedSpec().Major = atoiOrZero(ev.Values["MAJOR"])
			dev.TypedSpec().Minor = atoiOrZero(ev.Values["MINOR"])
			dev.TypedSpec().PartitionName = ev.Values["PARTNAME"]
			dev.TypedSpec().PartitionNumber = atoiOrZero(ev.Values["PARTN"])

			dev.TypedSpec().DevicePath = ev.DevicePath

			if dev.TypedSpec().Type == "partition" {
				dev.TypedSpec().Parent = filepath.Base(filepath.Dir(dev.TypedSpec().DevicePath))
				dev.TypedSpec().Secondaries = nil
			} else {
				dev.TypedSpec().Parent = ""
				dev.TypedSpec().Secondaries = sysblock.ReadSecondaries(ev.DevicePath)
			}

			dev.TypedSpec().Generation++

			return nil
		}); err != nil {
			return fmt.Errorf("failed to modify device %q: %w", id, err)
		}

		if err := inotifyWatcher.Add(devPath); err != nil {
			return fmt.Errorf("failed to add inotify watch for %q: %w", devPath, err)
		}
	case kobject.ActionRemove:
		if reStatErr == nil { // entry still exists, skip removing
			logger.Debug("skipped, as device path still exists")

			return nil
		}

		if err := r.Destroy(ctx, block.NewDevice(block.NamespaceName, id).Metadata()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to remove device %q: %w", id, err)
		}

		if err := inotifyWatcher.Remove(devPath); err != nil {
			logger.Debug("failed to remove inotify watch", zap.String("device", devPath), zap.Error(err))
		}
	default:
		logger.Debug("skipped, as action is not supported")
	}

	return nil
}

func atoiOrZero(s string) int {
	i, _ := strconv.Atoi(s) //nolint:errcheck

	return i
}
