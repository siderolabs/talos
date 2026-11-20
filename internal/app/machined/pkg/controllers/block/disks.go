// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"bufio"
	"cmp"
	"context"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	blkdev "github.com/siderolabs/go-blockdevice/v2/block"
	"github.com/siderolabs/go-cmd/pkg/cmd"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/sysblock"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// deviceMapperKindMultipath is the device-mapper kind of a multipath device (see DeviceMapperKind).
const deviceMapperKindMultipath = "mpath"

// DisksController provides a detailed view of blockdevices of type 'disk'.
type DisksController struct {
	// NativeMultipath is set when Talos assembles multipath devices itself (see MultipathController):
	// the disks which are going to be paths of a multipath device are never disks of their own then.
	NativeMultipath bool
}

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
		{
			Namespace: block.NamespaceName,
			Type:      block.SymlinkType,
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
	//
	// an entry is dropped as soon as the device is gone: a device which comes back under the same name
	// (e.g. device-mapper devices reuse "dm-N" names) starts counting generations from scratch, and
	// a stale entry would make it look unchanged and never probed.
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
		presentDevices := map[string]struct{}{}

		multipathPaths := sysblock.MultipathPaths(blockdevices.All(), ctrl.NativeMultipath)

		for device := range blockdevices.All() {
			if device.TypedSpec().Type != block.DeviceTypeDisk {
				continue
			}

			presentDevices[device.Metadata().ID()] = struct{}{}

			if device.TypedSpec().Major == 1 {
				// ignore ram disks (/dev/ramX), major number is 1
				// ref: https://www.kernel.org/doc/Documentation/admin-guide/devices.txt
				// ref: https://github.com/util-linux/util-linux/blob/c0207d354ee47fb56acfa64b03b5b559bb301280/misc-utils/lsblk.c#L2697-L2699
				continue
			}

			if multipath, ok := multipathPaths[device.Metadata().ID()]; ok {
				// a path of a multipath device is not a disk of its own, but a way to reach the
				// multipath device, so it is hidden; forget it, so that it is probed again once it
				// is no longer a path
				logger.Debug("hiding multipath path", zap.String("device", device.Metadata().ID()), zap.String("multipath", multipath))

				delete(lastObservedGenerations, device.Metadata().ID())

				continue
			}

			// always update symlinks, but skip if the disk hasn't been created yet
			if err = ctrl.updateSymlinks(ctx, r, device); err != nil {
				return err
			}

			if lastObserved, ok := lastObservedGenerations[device.Metadata().ID()]; ok && device.TypedSpec().Generation == lastObserved {
				// ignore disks which have same generation as before (don't query them once again)
				touchedDisks[device.Metadata().ID()] = struct{}{}

				continue
			}

			lastObservedGenerations[device.Metadata().ID()] = device.TypedSpec().Generation

			if err = ctrl.analyzeBlockDevice(ctx, r, logger.With(zap.String("device", device.Metadata().ID())), device, touchedDisks, blockdevices); err != nil {
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

		// forget the devices which are gone, whether or not they ever produced a disk
		maps.DeleteFunc(lastObservedGenerations, func(id string, _ int) bool {
			_, present := presentDevices[id]

			return !present
		})
	}
}

func (ctrl *DisksController) updateSymlinks(ctx context.Context, r controller.Runtime, device *block.Device) error {
	symlinks, err := safe.ReaderGetByID[*block.Symlink](ctx, r, device.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	_, err = safe.ReaderGetByID[*block.Disk](ctx, r, device.Metadata().ID())
	if err != nil {
		if state.IsNotFoundError(err) {
			// don't create disk entries even if we have symlinks, let analyze handle it
			return nil
		}

		return err
	}

	return safe.WriterModify(ctx, r, block.NewDisk(block.NamespaceName, device.Metadata().ID()), func(d *block.Disk) error {
		d.TypedSpec().Symlinks = symlinks.TypedSpec().Paths

		return nil
	})
}

//nolint:gocyclo
func (ctrl *DisksController) analyzeBlockDevice(
	ctx context.Context, r controller.Runtime, logger *zap.Logger, device *block.Device, touchedDisks map[string]struct{}, allBlockdevices safe.List[*block.Device],
) error {
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

		props = &blkdev.DeviceProperties{}
	}

	if props.DeviceMapperKind == deviceMapperKindMultipath {
		inheritPathProperties(ctx, logger, props, device.TypedSpec().Secondaries)
	}

	secondaryDisks := xslices.Map(device.TypedSpec().Secondaries, func(devID string) string {
		if secondary, ok := allBlockdevices.Find(func(dev *block.Device) bool {
			return dev.Metadata().ID() == devID
		}); ok {
			if secondary.TypedSpec().Parent != "" {
				return secondary.TypedSpec().Parent
			}
		}

		return devID
	})

	symlinks, err := safe.ReaderGetByID[*block.Symlink](ctx, r, device.Metadata().ID())
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	touchedDisks[device.Metadata().ID()] = struct{}{}

	serial := props.Serial
	if serial == "" {
		// try to get serial from udevd helpers
		serial = serialFromUdevdHelpers(ctx, device.Metadata().ID(), props.Transport)
		if serial == "" {
			logger.Debug("failed to get serial from udevd helpers, using empty value", zap.String("device", device.Metadata().ID()), zap.String("transport", props.Transport))
		}
	}

	return safe.WriterModify(ctx, r, block.NewDisk(block.NamespaceName, device.Metadata().ID()), func(d *block.Disk) error {
		d.TypedSpec().SetSize(size)

		d.TypedSpec().DevPath = filepath.Join("/dev", device.Metadata().ID())
		d.TypedSpec().IOSize = ioSize
		d.TypedSpec().SectorSize = sectorSize
		d.TypedSpec().Readonly = readOnly
		d.TypedSpec().CDROM = isCD

		d.TypedSpec().Model = props.Model
		d.TypedSpec().Serial = serial
		d.TypedSpec().Modalias = props.Modalias
		d.TypedSpec().WWID = props.WWID
		d.TypedSpec().UUID = props.UUID
		d.TypedSpec().BusPath = props.BusPath
		d.TypedSpec().SubSystem = props.SubSystem
		d.TypedSpec().Transport = props.Transport
		d.TypedSpec().Rotational = props.Rotational

		d.TypedSpec().DeviceMapperName = props.DeviceMapperName
		d.TypedSpec().DeviceMapperUUID = props.DeviceMapperUUID
		d.TypedSpec().DeviceMapperKind = props.DeviceMapperKind

		d.TypedSpec().SecondaryDisks = secondaryDisks

		if symlinks != nil {
			d.TypedSpec().Symlinks = symlinks.TypedSpec().Paths
		} else {
			d.TypedSpec().Symlinks = nil
		}

		d.TypedSpec().FirmwareVersion = props.FirmwareRevision

		return nil
	})
}

// inheritPathProperties fills in the properties a multipath device doesn't have from its paths.
//
// The paths of a multipath device are hidden, so the identity of the underlying disk (its model,
// serial number and WWID) would be lost otherwise, and a disk selector which used to match a path
// would no longer match anything. All paths lead to the same disk, so the first one which carries a
// property is as good as any.
func inheritPathProperties(ctx context.Context, logger *zap.Logger, props *blkdev.DeviceProperties, paths []string) {
	for _, path := range paths {
		if props.Model != "" && props.Serial != "" && props.WWID != "" {
			return
		}

		pathProps, err := func() (*blkdev.DeviceProperties, error) {
			bd, err := blkdev.NewFromPath(filepath.Join("/dev", path))
			if err != nil {
				return nil, err
			}

			defer bd.Close() //nolint:errcheck

			return bd.GetProperties()
		}()
		if err != nil {
			logger.Debug("failed to get properties of multipath path", zap.String("path", path), zap.Error(err))

			continue
		}

		if props.Serial == "" && pathProps.Serial == "" {
			// the paths of a SAN disk are most often SCSI devices, which only report the serial via VPD
			pathProps.Serial = serialFromUdevdHelpers(ctx, path, pathProps.Transport)
		}

		props.Model = cmp.Or(props.Model, pathProps.Model)
		props.Serial = cmp.Or(props.Serial, pathProps.Serial)
		props.WWID = cmp.Or(props.WWID, pathProps.WWID)
	}
}

func serialFromUdevdHelpers(ctx context.Context, id, transport string) string {
	switch strings.ToLower(transport) {
	case "ata":
		return runUdevdHelper(
			ctx,
			"/usr/lib/udev/ata_id",
			"--export", filepath.Join("/dev", id),
		)

	case "scsi":
		return runUdevdHelper(
			ctx,
			"/usr/lib/udev/scsi_id",
			"--export", "--allowlisted", "--device", filepath.Join("/dev", id),
		)

	default:
		return ""
	}
}

func runUdevdHelper(ctx context.Context, helper string, args ...string) string {
	out, err := cmd.RunWithOptions(ctx, helper, args)
	if err != nil {
		return ""
	}

	env := parseEnv(strings.NewReader(out))

	return cmp.Or(
		env["ID_SERIAL"],
		env["ID_SERIAL_SHORT"],
	)
}

func parseEnv(r io.Reader) map[string]string {
	env := map[string]string{}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		env[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	return env
}
