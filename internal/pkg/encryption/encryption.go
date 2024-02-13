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
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/go-blockdevice/blockdevice"
	"github.com/siderolabs/go-blockdevice/blockdevice/encryption"
	"github.com/siderolabs/go-blockdevice/blockdevice/encryption/luks"
	"github.com/siderolabs/go-blockdevice/blockdevice/encryption/token"
	"github.com/siderolabs/go-blockdevice/blockdevice/partition/gpt"
	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/internal/pkg/encryption/helpers"
	"github.com/siderolabs/talos/internal/pkg/encryption/keys"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

const (
	keyFetchTimeout   = time.Minute * 5
	keyHandlerTimeout = time.Second * 10
)

// NewHandler creates new Handler.
func NewHandler(device *blockdevice.BlockDevice, partition *gpt.Partition, encryptionConfig config.Encryption, getSystemInformation helpers.SystemInformationGetter) (*Handler, error) {
	var provider encryption.Provider

	switch encryptionConfig.Provider() {
	case encryption.LUKS2:
		cipher, err := luks.ParseCipherKind(encryptionConfig.Cipher())
		if err != nil {
			return nil, err
		}

		opts := []luks.Option{}
		if encryptionConfig.KeySize() != 0 {
			opts = append(opts, luks.WithKeySize(encryptionConfig.KeySize()))
		}

		if encryptionConfig.BlockSize() != 0 {
			opts = append(opts, luks.WithBlockSize(encryptionConfig.BlockSize()))
		}

		if encryptionConfig.Options() != nil {
			for _, opt := range encryptionConfig.Options() {
				if err = luks.ValidatePerfOption(opt); err != nil {
					return nil, err
				}
			}

			opts = append(opts, luks.WithPerfOptions(encryptionConfig.Options()...))
		}

		provider = luks.New(
			cipher,
			opts...,
		)
	default:
		return nil, fmt.Errorf("unknown encryption kind %s", encryptionConfig.Provider())
	}

	return &Handler{
		device:               device,
		partition:            partition,
		encryptionConfig:     encryptionConfig,
		encryptionProvider:   provider,
		getSystemInformation: getSystemInformation,
	}, nil
}

// Handler reads encryption config, creates appropriate
// encryption provider, handles encrypted partition open and close.
type Handler struct {
	device               *blockdevice.BlockDevice
	partition            *gpt.Partition
	encryptionConfig     config.Encryption
	encryptionProvider   encryption.Provider
	getSystemInformation helpers.SystemInformationGetter
	encryptedPath        string
}

// Open encrypted partition.
//
//nolint:gocyclo
func (h *Handler) Open(ctx context.Context) (string, error) {
	partPath, err := h.partition.Path()
	if err != nil {
		return "", err
	}

	sb, err := h.partition.SuperBlock()
	if err != nil {
		return "", err
	}

	handlers, err := h.initKeyHandlers(h.encryptionConfig, h.partition)
	if err != nil {
		return "", err
	}

	// encrypt if partition is not encrypted and empty
	if sb == nil {
		err = h.formatAndEncrypt(ctx, partPath, handlers)
		if err != nil {
			return "", err
		}
	} else if sb.Type() != h.encryptionConfig.Provider() {
		return "", fmt.Errorf("failed to encrypt the partition %s, because it is not empty", partPath)
	}

	var (
		path string
		key  *encryption.Key
	)

	if err = h.tryHandlers(ctx, handlers, func(ctx context.Context, handler keys.Handler) error {
		var token token.Token

		token, err = h.readToken(partPath, handler.Slot())
		if err != nil {
			return err
		}

		if key, err = handler.GetKey(ctx, token); err != nil {
			return err
		}

		path, err = h.encryptionProvider.Open(partPath, key)
		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return "", fmt.Errorf("failed to open encrypted device %s: %w", partPath, err)
	}

	log.Printf("mapped encrypted partition %s -> %s", partPath, path)

	if err = h.syncKeys(ctx, partPath, handlers, key); err != nil {
		return "", err
	}

	h.encryptedPath = path

	return path, nil
}

// Close encrypted partition.
func (h *Handler) Close() error {
	if h.encryptedPath == "" {
		return nil
	}

	if err := h.encryptionProvider.Close(h.encryptedPath); err != nil {
		return err
	}

	log.Printf("closed encrypted partition %s", h.encryptedPath)

	return nil
}

func (h *Handler) formatAndEncrypt(ctx context.Context, path string, handlers []keys.Handler) error {
	log.Printf("encrypting the partition %s (%s)", path, h.partition.Name)

	if len(handlers) == 0 {
		return errors.New("no encryption keys found")
	}

	var (
		key   *encryption.Key
		token token.Token
		err   error
	)

	if err = h.tryHandlers(ctx, handlers, func(ctx context.Context, h keys.Handler) error {
		if key, token, err = h.NewKey(ctx); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	if err = h.encryptionProvider.Encrypt(path, key); err != nil {
		return err
	}

	if token != nil {
		if err = h.encryptionProvider.SetToken(path, key.Slot, token); err != nil {
			return err
		}
	}

	for _, handler := range handlers {
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
func (h *Handler) syncKeys(ctx context.Context, path string, handlers []keys.Handler, k *encryption.Key) error {
	keyslots, err := h.encryptionProvider.ReadKeyslots(path)
	if err != nil {
		return err
	}

	visited := map[string]bool{}

	for _, handler := range handlers {
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

			log.Printf("updated encryption key at slot %d", handler.Slot())
		} else {
			// keyslot does not exist so just add the key
			if err = h.addKey(ctx, path, k, handler); err != nil {
				return err
			}

			log.Printf("added encryption key to slot %d", handler.Slot())
		}
	}

	// cleanup deleted key slots
	for slot := range keyslots.Keyslots {
		if !visited[slot] {
			s, err := strconv.ParseInt(slot, 10, 64)
			if err != nil {
				return err
			}

			if err = h.encryptionProvider.RemoveKey(path, int(s), k); err != nil {
				return err
			}

			log.Printf("removed key at slot %d", k.Slot)
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
	err = h.encryptionProvider.RemoveKey(path, handler.Slot(), existingKey)
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
	token, err := h.readToken(path, handler.Slot())
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

	return h.encryptionProvider.CheckKey(path, key)
}

func (h *Handler) addKey(ctx context.Context, path string, existingKey *encryption.Key, handler keys.Handler) error {
	key, token, err := handler.NewKey(ctx)
	if err != nil {
		return err
	}

	if token != nil {
		if err = h.encryptionProvider.SetToken(path, key.Slot, token); err != nil {
			return err
		}
	}

	err = h.encryptionProvider.AddKey(path, existingKey, key)
	if err != nil {
		return fmt.Errorf("failed to add new key during key update %w", err)
	}

	return nil
}

func (h *Handler) initKeyHandlers(encryptionConfig config.Encryption, partition *gpt.Partition) ([]keys.Handler, error) {
	handlers := make([]keys.Handler, 0, len(encryptionConfig.Keys()))

	for _, cfg := range encryptionConfig.Keys() {
		handler, err := keys.NewHandler(cfg, keys.WithPartitionLabel(partition.Name), keys.WithSystemInformationGetter(h.getSystemInformation))
		if err != nil {
			return nil, err
		}

		handlers = append(handlers, handler)
	}

	//nolint:scopelint
	sort.Slice(handlers, func(i, j int) bool { return handlers[i].Slot() < handlers[j].Slot() })

	return handlers, nil
}

func (h *Handler) tryHandlers(ctx context.Context, handlers []keys.Handler, cb func(ctx context.Context, h keys.Handler) error) error {
	callback := func(ctx context.Context, h keys.Handler) error {
		ctx, cancel := context.WithTimeout(ctx, keyHandlerTimeout)
		defer cancel()

		return cb(ctx, h)
	}

	return retry.Exponential(keyFetchTimeout, retry.WithUnits(time.Second), retry.WithJitter(time.Second)).RetryWithContext(ctx,
		func(ctx context.Context) error {
			var errs error

			for _, h := range handlers {
				if err := callback(ctx, h); err != nil {
					errs = multierror.Append(errs, err)

					log.Printf("failed to call key handler at slot %d: %s", h.Slot(), err)

					continue
				}

				return nil
			}

			return retry.ExpectedErrorf("no handlers available to get encryption keys from: %w", errs)
		})
}

func (h *Handler) readToken(path string, id int) (token.Token, error) {
	token := luks.Token[json.RawMessage]{}

	err := h.encryptionProvider.ReadToken(path, id, &token)
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
