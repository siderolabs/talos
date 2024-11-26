// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-blockdevice/v2/blkid"
	blockdev "github.com/siderolabs/go-blockdevice/v2/block"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/encryption"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// HandleEncryption makes sure the encryption for the volumes is handled appropriately.
func HandleEncryption(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext) error {
	getSystemInformation := func(ctx context.Context) (*hardware.SystemInformation, error) {
		if volumeContext.SystemInformation != nil {
			return volumeContext.SystemInformation, nil
		}

		return nil, fmt.Errorf("system information not available")
	}

	switch volumeContext.Cfg.TypedSpec().Encryption.Provider {
	case block.EncryptionProviderNone:
		// nothing to do
		volumeContext.Status.Phase = block.VolumePhasePrepared
		volumeContext.Status.MountLocation = volumeContext.Status.Location
		volumeContext.Status.EncryptionProvider = block.EncryptionProviderNone

		return nil
	case block.EncryptionProviderLUKS2:
		encryptionConfig := volumeContext.Cfg.TypedSpec().Encryption

		handler, err := encryption.NewHandler(encryptionConfig, volumeContext.Cfg.Metadata().ID(), getSystemInformation)
		if err != nil {
			return fmt.Errorf("failed to create encryption handler: %w", err)
		}

		return HandleEncryptionWithHandler(ctx, logger, volumeContext, handler)
	default:
		return fmt.Errorf("provider %s not implemented yet", volumeContext.Cfg.TypedSpec().Encryption.Provider)
	}
}

const encryptionTimeout = time.Minute

// HandleEncryptionWithHandler makes sure the encryption for the volumes is handled appropriately.
func HandleEncryptionWithHandler(ctx context.Context, logger *zap.Logger, volumeContext ManagerContext, handler *encryption.Handler) error {
	ctx, cancel := context.WithTimeout(ctx, encryptionTimeout)
	defer cancel()

	// lock either the parent device or the device itself
	devPath := volumeContext.Status.ParentLocation
	if devPath == "" {
		devPath = volumeContext.Status.Location
	}

	dev, err := blockdev.NewFromPath(devPath, blockdev.OpenForWrite())
	if err != nil {
		return xerrors.NewTaggedf[Retryable]("error opening disk: %w", err)
	}

	defer dev.Close() //nolint:errcheck

	if err = dev.RetryLockWithTimeout(ctx, true, 10*time.Second); err != nil {
		return xerrors.NewTaggedf[Retryable]("error locking disk: %w", err)
	}

	defer dev.Unlock() //nolint:errcheck

	info, err := blkid.ProbePath(volumeContext.Status.Location, blkid.WithSkipLocking(true))
	if err != nil {
		return xerrors.NewTaggedf[Retryable]("error probing disk: %w", err)
	}

	switch {
	case info.Name == "":
		// no filesystem, encrypt
		logger.Info("formatting and encrypting volume")

		if err = handler.FormatAndEncrypt(ctx, logger, volumeContext.Status.Location); err != nil {
			return xerrors.NewTaggedf[Retryable]("error formatting and encrypting volume: %w", err)
		}
	case info.Name == "luks":
		// already encrypted
	default:
		// mismatch
		return fmt.Errorf("block dev type mismatch: %s != %s", info.Name, "luks")
	}

	logger.Info("opening encrypted volume")

	encryptedName := filepath.Base(volumeContext.Status.Location) + "-encrypted"

	encryptedPath, err := handler.Open(ctx, logger, volumeContext.Status.Location, encryptedName)
	if err != nil {
		return xerrors.NewTaggedf[Retryable]("error opening encrypted volume: %w", err)
	}

	encryptedPath, err = filepath.EvalSymlinks(encryptedPath)
	if err != nil {
		return fmt.Errorf("error resolving symlink: %w", err)
	}

	volumeContext.Status.Phase = block.VolumePhasePrepared
	volumeContext.Status.MountLocation = encryptedPath
	volumeContext.Status.EncryptionProvider = volumeContext.Cfg.TypedSpec().Encryption.Provider

	return nil
}
