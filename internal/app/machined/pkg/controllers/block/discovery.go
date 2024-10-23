// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	"github.com/siderolabs/go-blockdevice/v2/partitioning"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// DiscoveryController provides a filesystem/partition discovery for blockdevices.
type DiscoveryController struct{}

// Name implements controller.Controller interface.
func (ctrl *DiscoveryController) Name() string {
	return "block.DiscoveryController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DiscoveryController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.DeviceType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveryRefreshRequestType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DiscoveryController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.DiscoveredVolumeType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: block.DiscoveryRefreshStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *DiscoveryController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// lastObservedGenerations holds the last observed generation of each device.
	//
	// when the generation of a device changes, the device might have changed and might need to be re-probed.
	lastObservedGenerations := map[string]int{}

	// whenever new DiscoveryRefresh requests are received, the devices are re-probed.
	var lastObservedDiscoveryRefreshRequest int

	// nextRescan holds the pool of devices to be rescanned in the next batch.
	nextRescan := map[string]int{}

	rescanTicker := time.NewTicker(100 * time.Millisecond)
	defer rescanTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-rescanTicker.C:
			if len(nextRescan) == 0 {
				continue
			}

			logger.Debug("rescanning devices", zap.Strings("devices", maps.Keys(nextRescan)))

			if nextRescanBatch, err := ctrl.rescan(ctx, r, logger, maps.Keys(nextRescan)); err != nil {
				return fmt.Errorf("failed to rescan devices: %w", err)
			} else {
				nextRescan = map[string]int{}

				for id := range nextRescanBatch {
					nextRescan[id] = lastObservedGenerations[id]
				}
			}

			if err := safe.WriterModify(ctx, r, block.NewDiscoveryRefreshStatus(block.NamespaceName, block.RefreshID), func(status *block.DiscoveryRefreshStatus) error {
				status.TypedSpec().Request = lastObservedDiscoveryRefreshRequest

				return nil
			}); err != nil {
				return fmt.Errorf("failed to write discovery refresh status: %w", err)
			}
		case <-r.EventCh():
			refreshRequest, err := safe.ReaderGetByID[*block.DiscoveryRefreshRequest](ctx, r, block.RefreshID)
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("failed to get refresh request: %w", err)
			}

			if refreshRequest != nil && refreshRequest.TypedSpec().Request != lastObservedDiscoveryRefreshRequest {
				lastObservedDiscoveryRefreshRequest = refreshRequest.TypedSpec().Request

				// force re-probe all devices
				clear(lastObservedGenerations)
			}

			devices, err := safe.ReaderListAll[*block.Device](ctx, r)
			if err != nil {
				return fmt.Errorf("failed to list devices: %w", err)
			}

			parents := map[string]string{}
			allDevices := map[string]struct{}{}

			for device := range devices.All() {
				if device.TypedSpec().Major == 1 {
					// ignore ram disks (/dev/ramX), major number is 1
					// ref: https://www.kernel.org/doc/Documentation/admin-guide/devices.txt
					// ref: https://github.com/util-linux/util-linux/blob/c0207d354ee47fb56acfa64b03b5b559bb301280/misc-utils/lsblk.c#L2697-L2699
					continue
				}

				allDevices[device.Metadata().ID()] = struct{}{}

				if device.TypedSpec().Parent != "" {
					parents[device.Metadata().ID()] = device.TypedSpec().Parent
				}

				if device.TypedSpec().Generation == lastObservedGenerations[device.Metadata().ID()] {
					continue
				}

				nextRescan[device.Metadata().ID()] = device.TypedSpec().Generation
				lastObservedGenerations[device.Metadata().ID()] = device.TypedSpec().Generation
			}

			// remove child devices if the parent is marked for rescan
			for id := range nextRescan {
				if parent, ok := parents[id]; ok {
					if _, ok := nextRescan[parent]; ok {
						delete(nextRescan, id)
					}
				}
			}

			// if the device is removed, add it to the nextRescan, and remove from lastObservedGenerations
			for id := range lastObservedGenerations {
				if _, ok := allDevices[id]; !ok {
					nextRescan[id] = lastObservedGenerations[id]
					delete(lastObservedGenerations, id)
				}
			}
		}
	}
}

//nolint:gocyclo,cyclop
func (ctrl *DiscoveryController) rescan(ctx context.Context, r controller.Runtime, logger *zap.Logger, ids []string) (map[string]struct{}, error) {
	failedIDs := map[string]struct{}{}
	touchedIDs := map[string]struct{}{}
	nextRescan := map[string]struct{}{}

	for _, id := range ids {
		device, err := safe.ReaderGetByID[*block.Device](ctx, r, id)
		if err != nil {
			if state.IsNotFoundError(err) {
				failedIDs[id] = struct{}{}

				continue
			}

			return nil, fmt.Errorf("failed to get device: %w", err)
		}

		info, err := blkid.ProbePath(filepath.Join("/dev", id), blkid.WithProbeLogger(logger.With(zap.String("device", id))))
		if err != nil {
			if errors.Is(err, blkid.ErrFailedLock) {
				// failed to lock the blockdevice, retry later
				logger.Debug("failed to lock device, retrying later", zap.String("id", id))

				nextRescan[id] = struct{}{}
			} else {
				logger.Debug("failed to probe device", zap.String("id", id), zap.Error(err))

				failedIDs[id] = struct{}{}
			}

			continue
		}

		logger.Debug("probed device", zap.String("id", id), zap.Any("info", info))

		if err = safe.WriterModify(ctx, r, block.NewDiscoveredVolume(block.NamespaceName, id), func(dv *block.DiscoveredVolume) error {
			dv.TypedSpec().DevPath = filepath.Join("/dev", id)
			dv.TypedSpec().Type = device.TypedSpec().Type
			dv.TypedSpec().DevicePath = device.TypedSpec().DevicePath
			dv.TypedSpec().Parent = device.TypedSpec().Parent

			if device.TypedSpec().Parent != "" {
				dv.TypedSpec().ParentDevPath = filepath.Join("/dev", device.TypedSpec().Parent)
			}

			dv.TypedSpec().SetSize(info.Size)
			dv.TypedSpec().SectorSize = info.SectorSize
			dv.TypedSpec().IOSize = info.IOSize

			ctrl.fillDiscoveredVolumeFromInfo(dv, info.ProbeResult)

			return nil
		}); err != nil {
			return nil, fmt.Errorf("failed to write discovered volume: %w", err)
		}

		touchedIDs[id] = struct{}{}

		for _, nested := range info.Parts {
			partID := partitioning.DevName(id, nested.PartitionIndex)

			if err = safe.WriterModify(ctx, r, block.NewDiscoveredVolume(block.NamespaceName, partID), func(dv *block.DiscoveredVolume) error {
				dv.TypedSpec().Type = "partition"
				dv.TypedSpec().DevPath = filepath.Join("/dev", partID)
				dv.TypedSpec().DevicePath = filepath.Join(device.TypedSpec().DevicePath, partID)
				dv.TypedSpec().Parent = id
				dv.TypedSpec().ParentDevPath = filepath.Join("/dev", id)

				if nested.ProbedSize != 0 {
					dv.TypedSpec().SetSize(nested.ProbedSize)
				} else {
					dv.TypedSpec().SetSize(nested.PartitionSize)
				}

				dv.TypedSpec().SectorSize = info.SectorSize
				dv.TypedSpec().IOSize = info.IOSize

				ctrl.fillDiscoveredVolumeFromInfo(dv, nested.ProbeResult)

				if nested.PartitionUUID != nil {
					dv.TypedSpec().PartitionUUID = nested.PartitionUUID.String()
				} else {
					dv.TypedSpec().PartitionUUID = ""
				}

				if nested.PartitionType != nil {
					dv.TypedSpec().PartitionType = nested.PartitionType.String()
				} else {
					dv.TypedSpec().PartitionType = ""
				}

				if nested.PartitionLabel != nil {
					dv.TypedSpec().PartitionLabel = *nested.PartitionLabel
				} else {
					dv.TypedSpec().PartitionLabel = ""
				}

				dv.TypedSpec().PartitionIndex = nested.PartitionIndex

				return nil
			}); err != nil {
				return nil, fmt.Errorf("failed to write discovered volume: %w", err)
			}

			touchedIDs[partID] = struct{}{}
		}
	}

	// clean up discovered volumes
	discoveredVolumes, err := safe.ReaderListAll[*block.DiscoveredVolume](ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to list discovered volumes: %w", err)
	}

	for dv := range discoveredVolumes.All() {
		if _, ok := touchedIDs[dv.Metadata().ID()]; ok {
			continue
		}

		_, isFailed := failedIDs[dv.Metadata().ID()]

		parentTouched := false

		if dv.TypedSpec().Parent != "" {
			if _, ok := touchedIDs[dv.TypedSpec().Parent]; ok {
				parentTouched = true
			}
		}

		if isFailed || parentTouched {
			// if the probe failed, or if the parent was touched, while this device was not, remove it
			if err = r.Destroy(ctx, dv.Metadata()); err != nil {
				return nil, fmt.Errorf("failed to destroy discovered volume: %w", err)
			}
		}
	}

	return nextRescan, nil
}

func (ctrl *DiscoveryController) fillDiscoveredVolumeFromInfo(dv *block.DiscoveredVolume, info blkid.ProbeResult) {
	dv.TypedSpec().Name = info.Name

	if info.UUID != nil {
		dv.TypedSpec().UUID = info.UUID.String()
	} else {
		dv.TypedSpec().UUID = ""
	}

	if info.Label != nil {
		dv.TypedSpec().Label = *info.Label
	} else {
		dv.TypedSpec().Label = ""
	}

	dv.TypedSpec().BlockSize = info.BlockSize
	dv.TypedSpec().FilesystemBlockSize = info.FilesystemBlockSize
	dv.TypedSpec().ProbedSize = info.ProbedSize
}
