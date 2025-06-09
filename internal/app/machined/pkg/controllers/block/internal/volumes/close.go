// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/encryption"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// Close the encrypted volumes.
func Close(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext) error {
	switch volumeContext.Cfg.TypedSpec().Type {
	case block.VolumeTypeTmpfs, block.VolumeTypeDirectory, block.VolumeTypeSymlink, block.VolumeTypeOverlay:
		// tmpfs, directory, symlink and overlay volumes can be always closed
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

		handler, err := encryption.NewHandler(encryptionConfig, volumeContext.Cfg.Metadata().ID(), volumeContext.GetSystemInformation, volumeContext.TPMLocker)
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

	encryptedName := filepath.Base(volumeContext.Status.Location) + "-encrypted"

	if err := handler.Close(ctx, encryptedName); err != nil {
		return xerrors.NewTaggedf[Retryable]("error closing encrypted volume %q: %w", encryptedName, err)
	}

	volumeContext.Status.Phase = block.VolumePhaseClosed

	logger.Info("encrypted volume closed", zap.String("name", encryptedName))

	return nil
}
