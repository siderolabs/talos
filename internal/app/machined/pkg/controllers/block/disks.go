// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	blkdev "github.com/siderolabs/go-blockdevice/v2/block"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// DisksController provides a detailed view of blockdevices of type 'disk'.
type DisksController struct{}

// Name implements controller.Controller interface.
func (ctrl *DisksController) Name() string {
	return "block.DisksController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DisksController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.DeviceType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DisksController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.DiskType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *DisksController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// lastObservedGenerations holds the last observed generation of each device.
	//
	// when the generation of a device changes, the device might have changed and might need to be re-probed.
	lastObservedGenerations := map[string]int{}

	for {
		select {
		case <-r.EventCh():
		case <-ctx.Done():
			return nil
		}

		blockdevices, err := safe.ReaderListAll[*block.Device](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list block devices: %w", err)
		}

		touchedDisks := map[string]struct{}{}

		for device := range blockdevices.All() {
			if device.TypedSpec().Type != "disk" {
				continue
			}

			if device.TypedSpec().Major == 1 {
				// ignore ram disks (/dev/ramX), major number is 1
				// ref: https://www.kernel.org/doc/Documentation/admin-guide/devices.txt
				// ref: https://github.com/util-linux/util-linux/blob/c0207d354ee47fb56acfa64b03b5b559bb301280/misc-utils/lsblk.c#L2697-L2699
				continue
			}

			if lastObserved, ok := lastObservedGenerations[device.Metadata().ID()]; ok && device.TypedSpec().Generation == lastObserved {
				// ignore disks which have some generation as before (don't query them once again)
				touchedDisks[device.Metadata().ID()] = struct{}{}

				continue
			}

			lastObservedGenerations[device.Metadata().ID()] = device.TypedSpec().Generation

			if err = ctrl.analyzeBlockDevice(ctx, r, logger.With(zap.String("device", device.Metadata().ID())), device, touchedDisks); err != nil {
				return fmt.Errorf("failed to analyze block device: %w", err)
			}
		}

		disks, err := safe.ReaderListAll[*block.Disk](ctx, r)
		if err != nil {
			return fmt.Errorf("failed to list disks: %w", err)
		}

		for disk := range disks.All() {
			if _, ok := touchedDisks[disk.Metadata().ID()]; ok {
				continue
			}

			if err = r.Destroy(ctx, disk.Metadata()); err != nil {
				return fmt.Errorf("failed to remove disk: %w", err)
			}

			delete(lastObservedGenerations, disk.Metadata().ID())
		}
	}
}

func (ctrl *DisksController) analyzeBlockDevice(ctx context.Context, r controller.Runtime, logger *zap.Logger, device *block.Device, touchedDisks map[string]struct{}) error {
	bd, err := blkdev.NewFromPath(filepath.Join("/dev", device.Metadata().ID()))
	if err != nil {
		logger.Debug("failed to open blockdevice", zap.Error(err))

		return nil
	}

	defer bd.Close() //nolint:errcheck

	size, err := bd.GetSize()
	if err != nil || size == 0 {
		return nil
	}

	if privateDM, _ := bd.IsPrivateDeviceMapper(); privateDM { //nolint:errcheck
		return nil
	}

	isCD := bd.IsCD()
	if isCD && bd.IsCDNoMedia() {
		// Linux reports non-zero size for CD-ROMs even when there is no media.
		size = 0
	}

	ioSize, err := bd.GetIOSize()
	if err != nil {
		logger.Debug("failed to get io size", zap.Error(err))
	}

	sectorSize := bd.GetSectorSize()

	readOnly, err := bd.IsReadOnly()
	if err != nil {
		logger.Debug("failed to get read only", zap.Error(err))
	}

	props, err := bd.GetProperties()
	if err != nil {
		logger.Debug("failed to get properties", zap.Error(err))
	}

	touchedDisks[device.Metadata().ID()] = struct{}{}

	return safe.WriterModify(ctx, r, block.NewDisk(block.NamespaceName, device.Metadata().ID()), func(d *block.Disk) error {
		d.TypedSpec().SetSize(size)

		d.TypedSpec().DevPath = filepath.Join("/dev", device.Metadata().ID())
		d.TypedSpec().IOSize = ioSize
		d.TypedSpec().SectorSize = sectorSize
		d.TypedSpec().Readonly = readOnly
		d.TypedSpec().CDROM = isCD

		d.TypedSpec().Model = props.Model
		d.TypedSpec().Serial = props.Serial
		d.TypedSpec().Modalias = props.Modalias
		d.TypedSpec().WWID = props.WWID
		d.TypedSpec().BusPath = props.BusPath
		d.TypedSpec().SubSystem = props.SubSystem
		d.TypedSpec().Transport = props.Transport
		d.TypedSpec().Rotational = props.Rotational

		return nil
	})
}
