// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package encryption provides modules for the partition encryption handling.
package encryption

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-blockdevice/v2/encryption/luks"
	"github.com/siderolabs/go-blockdevice/v2/encryption/token"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/encryption/helpers"
	"github.com/siderolabs/talos/internal/pkg/encryption/keys"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

const keyHandlerTimeout = time.Second * 10

// NewHandler creates new Handler.
func NewHandler(encryptionConfig block.EncryptionSpec, volumeID string, getSystemInformation helpers.SystemInformationGetter) (*Handler, error) {
	cipher, err := luks.ParseCipherKind(encryptionConfig.Cipher)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cipher kind: %w", err)
	}

	var opts []luks.Option

	if encryptionConfig.KeySize != 0 {
		opts = append(opts, luks.WithKeySize(encryptionConfig.KeySize))
	}

	if encryptionConfig.BlockSize != 0 {
		opts = append(opts, luks.WithBlockSize(encryptionConfig.BlockSize))
	}

	if encryptionConfig.PerfOptions != nil {
		for _, opt := range encryptionConfig.PerfOptions {
			if err = luks.ValidatePerfOption(opt); err != nil {
				return nil, fmt.Errorf("invalid luks performance options: %w", err)
			}
		}

		opts = append(opts, luks.WithPerfOptions(encryptionConfig.PerfOptions...))
	}

	keyHandlers := make([]keys.Handler, 0, len(encryptionConfig.Keys))

	for _, cfg := range encryptionConfig.Keys {
		handler, err := keys.NewHandler(cfg, keys.WithVolumeID(volumeID), keys.WithSystemInformationGetter(getSystemInformation))
		if err != nil {
			return nil, err
		}

		keyHandlers = append(keyHandlers, handler)
	}

	//nolint:scopelint
	sort.Slice(keyHandlers, func(i, j int) bool { return keyHandlers[i].Slot() < keyHandlers[j].Slot() })

	provider := luks.New(
		cipher,
		opts...,
	)

	return &Handler{
		encryptionProvider: provider,
		keyHandlers:        keyHandlers,
	}, nil
}

// Handler reads encryption config, creates appropriate
// encryption provider, handles encrypted partition open and close.
type Handler struct {
	encryptionProvider encryption.Provider
	keyHandlers        []keys.Handler
}

// Open encrypted partition.
func (h *Handler) Open(ctx context.Context, logger *zap.Logger, devicePath, encryptedName string) (string, error) {
	var (
		path string
		key  *encryption.Key
	)

	if err := h.tryHandlers(ctx, logger, func(ctx context.Context, handler keys.Handler) error {
		token, err := h.readToken(ctx, devicePath, handler.Slot())
		if err != nil {
			return err
		}

		if key, err = handler.GetKey(ctx, token); err != nil {
			return err
		}

		path, err = h.encryptionProvider.Open(ctx, devicePath, encryptedName, key)
		if err != nil {
			return err
		}

		logger.Info("opened encrypted device", zap.Int("slot", handler.Slot()), zap.String("type", fmt.Sprintf("%T", handler)))

		return nil
	}); err != nil {
		return "", fmt.Errorf("failed to open encrypted device %s: %w", devicePath, err)
	}

	if err := h.syncKeys(ctx, logger, devicePath, key); err != nil {
		return "", err
	}

	return path, nil
}

// Close encrypted partition.
func (h *Handler) Close(ctx context.Context, encryptedPath string) error {
	if err := h.encryptionProvider.Close(ctx, encryptedPath); err != nil {
		return fmt.Errorf("error closing %s: %w", encryptedPath, err)
	}

	return nil
}

// FormatAndEncrypt formats and encrypts the volume.
func (h *Handler) FormatAndEncrypt(ctx context.Context, logger *zap.Logger, path string) error {
	if len(h.keyHandlers) == 0 {
		return errors.New("no encryption keys found")
	}

	var (
		key   *encryption.Key
		token token.Token
		err   error
	)

	if err = h.tryHandlers(ctx, logger, func(ctx context.Context, h keys.Handler) error {
		if key, token, err = h.NewKey(ctx); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	if err = h.encryptionProvider.Encrypt(ctx, path, key); err != nil {
		return err
	}

	if token != nil {
		if err = h.encryptionProvider.SetToken(ctx, path, key.Slot, token); err != nil {
			return err
		}
	}

	for _, handler := range h.keyHandlers {
		if handler.Slot() == key.Slot {
			continue
		}

		if err := h.addKey(ctx, path, key, handler); err != nil {
			return err
		}
	}

	return nil
}

//nolint:gocyclo
func (h *Handler) syncKeys(ctx context.Context, logger *zap.Logger, path string, k *encryption.Key) error {
	keyslots, err := h.encryptionProvider.ReadKeyslots(path)
	if err != nil {
		return err
	}

	visited := map[string]bool{}

	for _, handler := range h.keyHandlers {
		slot := strconv.Itoa(handler.Slot())
		visited[slot] = true
		// no need to update the key which we already detected as unchanged
		if k.Slot == handler.Slot() {
			continue
		}

		// keyslot exists
		if _, ok := keyslots.Keyslots[slot]; ok {
			if err = h.updateKey(ctx, path, k, handler); err != nil {
				return err
			}

			logger.Info("updated encryption key", zap.Int("slot", handler.Slot()))
		} else {
			// keyslot does not exist so just add the key
			if err = h.addKey(ctx, path, k, handler); err != nil {
				return err
			}

			logger.Info("added encryption key", zap.Int("slot", handler.Slot()))
		}
	}

	// cleanup deleted key slots
	for slot := range keyslots.Keyslots {
		if !visited[slot] {
			s, err := strconv.ParseInt(slot, 10, 64)
			if err != nil {
				return err
			}

			if err = h.encryptionProvider.RemoveKey(ctx, path, int(s), k); err != nil {
				return err
			}

			logger.Info("removed encryption key", zap.Int("slot", k.Slot))
		}
	}

	return nil
}

func (h *Handler) updateKey(ctx context.Context, path string, existingKey *encryption.Key, handler keys.Handler) error {
	valid, err := h.checkKey(ctx, path, handler)
	if err != nil {
		return err
	}

	if valid {
		return nil
	}

	// re-add the key to the slot
	err = h.encryptionProvider.RemoveKey(ctx, path, handler.Slot(), existingKey)
	if err != nil {
		return fmt.Errorf("failed to drop old key during key update %w", err)
	}

	err = h.addKey(ctx, path, existingKey, handler)
	if err != nil {
		return fmt.Errorf("failed to add new key during key update %w", err)
	}

	return err
}

func (h *Handler) checkKey(ctx context.Context, path string, handler keys.Handler) (bool, error) {
	token, err := h.readToken(ctx, path, handler.Slot())
	if err != nil {
		return false, err
	}

	key, err := handler.GetKey(ctx, token)
	if err != nil {
		if errors.Is(err, keys.ErrTokenInvalid) {
			return false, nil
		}

		return false, err
	}

	return h.encryptionProvider.CheckKey(ctx, path, key)
}

func (h *Handler) addKey(ctx context.Context, path string, existingKey *encryption.Key, handler keys.Handler) error {
	key, token, err := handler.NewKey(ctx)
	if err != nil {
		return err
	}

	if token != nil {
		if err = h.encryptionProvider.SetToken(ctx, path, key.Slot, token); err != nil {
			return err
		}
	}

	err = h.encryptionProvider.AddKey(ctx, path, existingKey, key)
	if err != nil {
		return fmt.Errorf("failed to add new key during key update %w", err)
	}

	return nil
}

func (h *Handler) tryHandlers(ctx context.Context, logger *zap.Logger, cb func(ctx context.Context, h keys.Handler) error) error {
	callback := func(ctx context.Context, h keys.Handler) error {
		ctx, cancel := context.WithTimeout(ctx, keyHandlerTimeout)
		defer cancel()

		return cb(ctx, h)
	}

	var errs error

	for _, h := range h.keyHandlers {
		if err := callback(ctx, h); err != nil {
			errs = multierror.Append(errs, err)

			logger.Warn("failed to call key handler", zap.Int("slot", h.Slot()), zap.Error(err))

			continue
		}

		return nil
	}

	return fmt.Errorf("no handlers available to get encryption keys from: %w", errs)
}

func (h *Handler) readToken(ctx context.Context, path string, id int) (token.Token, error) {
	token := luks.Token[json.RawMessage]{}

	err := h.encryptionProvider.ReadToken(ctx, path, id, &token)
	if err != nil {
		if errors.Is(err, encryption.ErrTokenNotFound) {
			return nil, nil //nolint:nilnil
		}

		return nil, err
	}

	switch token.Type {
	case keys.TokenTypeKMS:
		kmsData := &keys.KMSToken{}

		if err = json.Unmarshal(token.UserData, &kmsData); err != nil {
			return nil, err
		}

		return &luks.Token[*keys.KMSToken]{
			Type:     token.Type,
			UserData: kmsData,
		}, nil
	case keys.TokenTypeTPM:
		tpmData := &keys.TPMToken{}

		if err = json.Unmarshal(token.UserData, &tpmData); err != nil {
			return nil, err
		}

		return &luks.Token[*keys.TPMToken]{
			Type:     token.Type,
			UserData: tpmData,
		}, nil
	default:
		return nil, fmt.Errorf("unknown token type %s", token.Type)
	}
}
