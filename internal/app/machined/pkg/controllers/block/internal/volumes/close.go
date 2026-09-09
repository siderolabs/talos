// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"context"
	"fmt"
	"os"

	"github.com/siderolabs/gen/xerrors"
	blockdev "github.com/siderolabs/go-blockdevice/v2/block"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/encryption"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// Close the encrypted volumes.
func Close(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext) error {
	switch volumeContext.Cfg.TypedSpec().Type {
	case block.VolumeTypeTmpfs, block.VolumeTypeDirectory, block.VolumeTypeSymlink, block.VolumeTypeOverlay, block.VolumeTypeExternal:
		// volume types can be always closed
		volumeContext.Status.Phase = block.VolumePhaseClosed

		return nil
	case block.VolumeTypeDisk, block.VolumeTypePartition:
	}

	switch volumeContext.Cfg.TypedSpec().Encryption.Provider {
	case block.EncryptionProviderNone:
		// nothing to do
		volumeContext.Status.Phase = block.VolumePhaseClosed

		return nil
	case block.EncryptionProviderLUKS2:
		encryptionConfig := volumeContext.Cfg.TypedSpec().Encryption

		handler, err := encryption.NewHandler(encryptionConfig, volumeContext.Cfg.Metadata().ID(), volumeContext.EncryptionHelpers)
		if err != nil {
			return fmt.Errorf("failed to create encryption handler: %w", err)
		}

		return CloseWithHandler(ctx, logger, volumeContext, handler)
	default:
		return fmt.Errorf("provider %s not implemented yet", volumeContext.Cfg.TypedSpec().Encryption.Provider)
	}
}

// CloseWithHandler closes the encrypted volumes.
func CloseWithHandler(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext, handler *encryption.Handler) error {
	ctx, cancel := context.WithTimeout(ctx, encryptionTimeout)
	defer cancel()

	mappedName := handler.Name() + "-" + volumeContext.Cfg.Metadata().ID()

	// partition maps stacked on the decrypted device hold it open, so they have to go first: the
	// kernel drops the partitions of a regular disk together with the disk, but the partitions of a
	// device-mapper disk are device-mapper devices of their own, which stay until they are removed
	if err := removePartitionMaps(volumeContext.Status.MountLocation); err != nil {
		return xerrors.NewTaggedf[Retryable]("error removing the partition maps of %q: %w", volumeContext.Status.MountLocation, err)
	}

	if err := handler.Close(ctx, mappedName); err != nil {
		return xerrors.NewTaggedf[Retryable]("error closing encrypted volume mapped to %q: %w", mappedName, err)
	}

	volumeContext.Status.Phase = block.VolumePhaseClosed

	logger.Info("encrypted volume closed", zap.String("name", mappedName))

	return nil
}

// removePartitionMaps removes the device-mapper partition maps of the device, if it has any.
//
// It does nothing for a device which is not device-mapper, and nothing for a device which is gone
// already, taking its maps with it.
func removePartitionMaps(devPath string) error {
	if devPath == "" {
		return nil
	}

	dev, err := blockdev.NewFromPath(devPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	defer dev.Close() //nolint:errcheck

	return dev.SyncPartitionMaps(nil)
}
