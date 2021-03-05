// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package encryption provides modules for the partition encryption handling.
package encryption

import (
	"fmt"
	"log"
	"sort"
	"strconv"

	"github.com/talos-systems/go-blockdevice/blockdevice"
	"github.com/talos-systems/go-blockdevice/blockdevice/encryption"
	"github.com/talos-systems/go-blockdevice/blockdevice/encryption/luks"
	"github.com/talos-systems/go-blockdevice/blockdevice/partition/gpt"

	"github.com/talos-systems/talos/internal/pkg/encryption/keys"
	"github.com/talos-systems/talos/pkg/machinery/config"
)

// NewHandler creates new Handler.
func NewHandler(device *blockdevice.BlockDevice, partition *gpt.Partition, encryptionConfig config.Encryption) (*Handler, error) {
	keys, err := getKeys(encryptionConfig, partition)
	if err != nil {
		return nil, err
	}

	var provider encryption.Provider

	switch encryptionConfig.Kind() {
	case encryption.LUKS2:
		cipher, err := luks.ParseCipherKind(encryptionConfig.Cipher())
		if err != nil {
			return nil, err
		}

		provider = luks.New(
			cipher,
		)
	default:
		return nil, fmt.Errorf("unknown encryption kind %s", encryptionConfig.Kind())
	}

	return &Handler{
		device:             device,
		partition:          partition,
		encryptionConfig:   encryptionConfig,
		keys:               keys,
		encryptionProvider: provider,
	}, nil
}

// Handler reads encryption config, creates appropriate
// encryption provider, handles encrypted partition open and close.
type Handler struct {
	device             *blockdevice.BlockDevice
	partition          *gpt.Partition
	encryptionConfig   config.Encryption
	keys               []*encryption.Key
	encryptionProvider encryption.Provider
	encryptedPath      string
}

// Open encrypted partition.
//nolint:gocyclo
func (h *Handler) Open() (string, error) {
	partPath, err := h.partition.Path()
	if err != nil {
		return "", err
	}

	sb, err := h.partition.SuperBlock()
	if err != nil {
		return "", err
	}

	var path string

	// encrypt if partition is not encrypted and empty
	if sb == nil {
		err = h.formatAndEncrypt(partPath)
		if err != nil {
			return "", err
		}
	} else if sb.Type() != h.encryptionConfig.Kind() {
		return "", fmt.Errorf("failed to encrypt the partition %s, because it is not empty", partPath)
	}

	var k *encryption.Key

	for _, k = range h.keys {
		path, err = h.encryptionProvider.Open(partPath, k)
		if err != nil {
			if err == encryption.ErrEncryptionKeyRejected {
				continue
			}

			return "", err
		}

		break
	}

	if path == "" {
		return "", fmt.Errorf("failed to open encrypted device %s, no key matched", partPath)
	}

	log.Printf("mapped encrypted partition %s -> %s", partPath, path)

	if err = h.syncKeys(k, partPath); err != nil {
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

func (h *Handler) formatAndEncrypt(path string) error {
	log.Printf("encrypting the partition %s (%s)", path, h.partition.Name)

	if len(h.keys) == 0 {
		return fmt.Errorf("no encryption keys found")
	}

	key := h.keys[0]

	err := h.encryptionProvider.Encrypt(path, key)
	if err != nil {
		return err
	}

	for _, extraKey := range h.keys[1:] {
		if err = h.encryptionProvider.AddKey(path, key, extraKey); err != nil {
			return err
		}
	}

	return nil
}

//nolint:gocyclo
func (h *Handler) syncKeys(k *encryption.Key, path string) error {
	keyslots, err := h.encryptionProvider.ReadKeyslots(path)
	if err != nil {
		return err
	}

	visited := map[string]bool{}

	for _, key := range h.keys {
		slot := fmt.Sprintf("%d", key.Slot)
		visited[slot] = true
		// no need to update the key which we already detected as unchanged
		if k.Slot == key.Slot {
			continue
		}

		// keyslot exists
		if _, ok := keyslots.Keyslots[slot]; ok {
			if err = h.updateKey(k, key, path); err != nil {
				return err
			}

			log.Printf("updated encryption key at slot %d", key.Slot)
		} else {
			// keyslot does not exist so just add the key
			if err = h.encryptionProvider.AddKey(path, k, key); err != nil {
				return err
			}

			log.Printf("added encryption key to slot %d", key.Slot)
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

func (h *Handler) updateKey(existingKey, newKey *encryption.Key, path string) error {
	if valid, err := h.encryptionProvider.CheckKey(path, newKey); err != nil {
		return err
	} else if !valid {
		// re-add the key to the slot
		err = h.encryptionProvider.RemoveKey(path, newKey.Slot, existingKey)
		if err != nil {
			return fmt.Errorf("failed to drop old key during key update %w", err)
		}

		err = h.encryptionProvider.AddKey(path, existingKey, newKey)
		if err != nil {
			return fmt.Errorf("failed to add new key during key update %w", err)
		}

		return err
	}

	return nil
}

func getKeys(encryptionConfig config.Encryption, partition *gpt.Partition) ([]*encryption.Key, error) {
	encryptionKeys := make([]*encryption.Key, len(encryptionConfig.Keys()))

	for i, cfg := range encryptionConfig.Keys() {
		handler, err := keys.NewHandler(cfg)
		if err != nil {
			return nil, err
		}

		k, err := handler.GetKey(keys.WithPartitionLabel(partition.Name))
		if err != nil {
			return nil, err
		}

		encryptionKeys[i] = encryption.NewKey(cfg.Slot(), k)
	}

	//nolint:scopelint
	sort.Slice(encryptionKeys, func(i, j int) bool { return encryptionKeys[i].Slot < encryptionKeys[j].Slot })

	return encryptionKeys, nil
}
