// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"fmt"
	"os"

	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	blockdev "github.com/siderolabs/go-blockdevice/v2/block"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/makefs"
)

// Format establishes a filesystem on a block device.
func Format(logger *zap.Logger, volumeContext ManagerContext) error {
	if volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type == block.FilesystemTypeNone {
		// nothing to do
		volumeContext.Status.Phase = block.VolumePhaseReady

		return nil
	}

	// lock either the parent device or the device itself
	devPath := volumeContext.Status.ParentLocation
	if devPath == "" {
		devPath = volumeContext.Status.Location
	}

	f, err := os.OpenFile(devPath, os.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return xerrors.NewTaggedf[Retryable]("error opening disk: %w", err)
	}

	defer f.Close() //nolint:errcheck

	dev := blockdev.NewFromFile(f)

	if err = dev.TryLock(true); err != nil {
		return xerrors.NewTaggedf[Retryable]("error locking disk: %w", err)
	}

	defer dev.Unlock() //nolint:errcheck

	info, err := blkid.ProbePath(volumeContext.Status.Location, blkid.WithSkipLocking(true))
	if err != nil {
		return xerrors.NewTaggedf[Retryable]("error probing disk: %w", err)
	}

	switch {
	case info.Name == "":
		// no filesystem, format
	case info.Name == volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type.String():
		// filesystem already exists
		volumeContext.Status.Phase = block.VolumePhaseReady

		return nil
	default:
		// mismatch
		return fmt.Errorf("filesystem type mismatch: %s != %s", info.Name, volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type)
	}

	switch volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type { //nolint:exhaustive
	case block.FilesystemTypeXFS:
		var makefsOptions []makefs.Option

		// xfs doesn't support by default filesystems < 300 MiB
		if volumeContext.Status.Size <= 300*1024*1024 {
			makefsOptions = append(makefsOptions, makefs.WithUnsupportedFSOption(true))
		}

		if err = makefs.XFS(volumeContext.Status.Location, makefsOptions...); err != nil {
			return fmt.Errorf("error formatting XFS: %w", err)
		}
	default:
		return fmt.Errorf("unsupported filesystem type: %s", volumeContext.Cfg.TypedSpec().Provisioning.FilesystemSpec.Type)
	}

	volumeContext.Status.Phase = block.VolumePhaseReady

	return nil
}
