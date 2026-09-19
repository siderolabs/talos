// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package encryption

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-blockdevice/v2/encryption/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/pkg/encryption/keys"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// fakeProvider is an in-memory encryption provider which tracks keyslots and the operations performed on them.
type fakeProvider struct {
	slots map[int][]byte

	encrypted bool
	removed   []int
	added     []int
}

func newFakeProvider(slots map[int][]byte) *fakeProvider {
	return &fakeProvider{slots: slots}
}

func (p *fakeProvider) Encrypt(_ context.Context, _ string, key *encryption.Key) error {
	p.encrypted = true
	p.slots[key.Slot] = key.Value

	return nil
}

func (p *fakeProvider) IsOpen(context.Context, string, string) (bool, string, error) {
	return false, "", nil
}

func (p *fakeProvider) Open(_ context.Context, _ string, mappedName string, key *encryption.Key) (string, error) {
	value, ok := p.slots[key.Slot]
	if !ok || string(value) != string(key.Value) {
		return "", encryption.ErrEncryptionKeyRejected
	}

	return "/dev/mapper/" + mappedName, nil
}

func (p *fakeProvider) Close(context.Context, string) error { return nil }

func (p *fakeProvider) AddKey(_ context.Context, _ string, _, newKey *encryption.Key) error {
	p.slots[newKey.Slot] = newKey.Value
	p.added = append(p.added, newKey.Slot)

	return nil
}

func (p *fakeProvider) CheckKey(_ context.Context, _ string, key *encryption.Key) (bool, error) {
	value, ok := p.slots[key.Slot]
	if !ok {
		return false, fmt.Errorf("slot %d does not exist", key.Slot)
	}

	return string(value) == string(key.Value), nil
}

func (p *fakeProvider) RemoveKey(_ context.Context, _ string, slot int, _ *encryption.Key) error {
	delete(p.slots, slot)
	p.removed = append(p.removed, slot)

	return nil
}

func (p *fakeProvider) ReadKeyslots(string) (*encryption.Keyslots, error) {
	keyslots := &encryption.Keyslots{Keyslots: map[string]*encryption.Keyslot{}}

	for slot := range p.slots {
		keyslots.Keyslots[strconv.Itoa(slot)] = &encryption.Keyslot{}
	}

	return keyslots, nil
}

func (p *fakeProvider) SetToken(context.Context, string, int, token.Token) error { return nil }

func (p *fakeProvider) ReadToken(context.Context, string, int, token.Token) error {
	return encryption.ErrTokenNotFound
}

func (p *fakeProvider) RemoveToken(context.Context, string, int) error { return nil }

// recoveryGetter returns a recovery key getter which returns the key only if it's set.
func recoveryGetter(key *[]byte) func(context.Context, string) ([]byte, bool, error) {
	return func(context.Context, string) ([]byte, bool, error) {
		if key == nil || *key == nil {
			return nil, false, nil
		}

		return *key, true, nil
	}
}

func staticHandler(t *testing.T, slot int, passphrase string) keys.Handler {
	t.Helper()

	handler, err := keys.NewHandler(block.EncryptionKey{
		Slot:             slot,
		Type:             block.EncryptionKeyStatic,
		StaticPassphrase: []byte(passphrase),
	})
	require.NoError(t, err)

	return handler
}

func recoveryHandler(t *testing.T, slot int, key *[]byte) keys.Handler {
	t.Helper()

	handler, err := keys.NewHandler(block.EncryptionKey{
		Slot: slot,
		Type: block.EncryptionKeyRecovery,
	}, keys.WithVolumeID("STATE"), keys.WithRecoveryKeyGetter(recoveryGetter(key)))
	require.NoError(t, err)

	return handler
}

func TestRecoveryKeyFormatAndEncrypt(t *testing.T) {
	t.Parallel()

	provider := newFakeProvider(map[int][]byte{})

	h := &Handler{
		encryptionProvider: provider,
		keyHandlers: []keys.Handler{
			staticHandler(t, 0, "static-key"),
			recoveryHandler(t, 1, nil),
		},
	}

	// the recovery key is not available at format time: the volume should still be formatted,
	// and the recovery slot left empty
	require.NoError(t, h.FormatAndEncrypt(t.Context(), zaptest.NewLogger(t), "/dev/null"))

	assert.True(t, provider.encrypted)
	assert.Len(t, provider.slots, 1)
	assert.Empty(t, provider.added)
}

func TestRecoveryKeySync(t *testing.T) {
	t.Parallel()

	const (
		unlockKey   = "unlock-key"
		recoveryKey = "correct horse battery staple"
	)

	unlockedWith := encryption.NewKey(0, []byte(unlockKey))

	for _, test := range []struct {
		name string

		recoverySlotEnrolled bool
		recoveryKeySupplied  bool

		expectedAdded []int
		expectedSlots int
	}{
		{
			name: "pending slot is left empty when the key is not supplied",

			expectedSlots: 1,
		},
		{
			name: "pending slot is enrolled when the key is supplied",

			recoveryKeySupplied: true,

			expectedAdded: []int{1},
			expectedSlots: 2,
		},
		{
			name: "enrolled slot is preserved when the key is not supplied",

			recoverySlotEnrolled: true,

			expectedSlots: 2,
		},
		{
			name: "enrolled slot is verified and left alone when the key is supplied",

			recoverySlotEnrolled: true,
			recoveryKeySupplied:  true,

			expectedSlots: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			slots := map[int][]byte{
				0: []byte(unlockKey),
			}

			if test.recoverySlotEnrolled {
				slots[1] = []byte(recoveryKey)
			}

			var suppliedKey []byte

			if test.recoveryKeySupplied {
				suppliedKey = []byte(recoveryKey)
			}

			provider := newFakeProvider(slots)

			h := &Handler{
				encryptionProvider: provider,
				keyHandlers: []keys.Handler{
					staticHandler(t, 0, unlockKey),
					recoveryHandler(t, 1, &suppliedKey),
				},
			}

			failedSyncs, err := h.syncKeys(t.Context(), zaptest.NewLogger(t), "/dev/null", unlockedWith)
			require.NoError(t, err)

			// recovery key state is never a failed sync
			assert.Empty(t, failedSyncs)
			assert.Equal(t, test.expectedAdded, provider.added)
			assert.Empty(t, provider.removed)
			assert.Len(t, provider.slots, test.expectedSlots)

			if test.expectedSlots == 2 {
				assert.Equal(t, []byte(recoveryKey), provider.slots[1])
			}
		})
	}
}

func TestRecoveryKeyOpen(t *testing.T) {
	t.Parallel()

	const recoveryKey = "correct horse battery staple"

	// the automatic key (slot 0) is stale: it doesn't match the slot anymore
	provider := newFakeProvider(map[int][]byte{
		0: []byte("old-key"),
		1: []byte(recoveryKey),
	})

	var suppliedKey []byte

	h := &Handler{
		encryptionProvider: provider,
		keyHandlers: []keys.Handler{
			staticHandler(t, 0, "new-key"),
			recoveryHandler(t, 1, &suppliedKey),
		},
	}

	// without the recovery key, the volume can't be opened
	_, _, _, err := h.Open(t.Context(), zaptest.NewLogger(t), "/dev/null", "luks-STATE")
	require.Error(t, err)
	require.ErrorIs(t, err, keys.ErrKeyNotAvailable)

	// once the operator supplies the key, the volume opens with the recovery slot,
	// and the stale automatic slot is re-enrolled
	suppliedKey = []byte(recoveryKey)

	path, usedSlot, failedSyncs, err := h.Open(t.Context(), zaptest.NewLogger(t), "/dev/null", "luks-STATE")
	require.NoError(t, err)

	assert.Equal(t, "/dev/mapper/luks-STATE", path)
	assert.Equal(t, 1, usedSlot)
	assert.Empty(t, failedSyncs)
	assert.Equal(t, []int{0}, provider.removed)
	assert.Equal(t, []int{0}, provider.added)
	assert.Equal(t, []byte("new-key"), provider.slots[0])
}

func TestSyncAndPendingSlots(t *testing.T) {
	t.Parallel()

	const (
		unlockKey   = "unlock-key"
		recoveryKey = "correct horse battery staple"
	)

	// the volume was formatted with the automatic key only, the recovery slot is pending
	provider := newFakeProvider(map[int][]byte{
		0: []byte(unlockKey),
	})

	var suppliedKey []byte

	h := &Handler{
		encryptionProvider: provider,
		keyHandlers: []keys.Handler{
			staticHandler(t, 0, unlockKey),
			recoveryHandler(t, 1, &suppliedKey),
		},
	}

	pending, err := h.PendingSlots("/dev/null")
	require.NoError(t, err)
	assert.Equal(t, []int{1}, pending)

	// sync without the recovery key: nothing to do
	failedSyncs, err := h.Sync(t.Context(), zaptest.NewLogger(t), "/dev/null")
	require.NoError(t, err)
	assert.Empty(t, failedSyncs)
	assert.Empty(t, provider.added)

	// the operator supplies the key: the recovery slot is enrolled using the automatic key as the existing key
	suppliedKey = []byte(recoveryKey)

	failedSyncs, err = h.Sync(t.Context(), zaptest.NewLogger(t), "/dev/null")
	require.NoError(t, err)
	assert.Empty(t, failedSyncs)
	assert.Equal(t, []int{1}, provider.added)
	assert.Equal(t, []byte(recoveryKey), provider.slots[1])

	pending, err = h.PendingSlots("/dev/null")
	require.NoError(t, err)
	assert.Empty(t, pending)
}

func TestSyncNoValidKey(t *testing.T) {
	t.Parallel()

	// none of the configured keys matches the volume
	provider := newFakeProvider(map[int][]byte{
		0: []byte("old-key"),
	})

	h := &Handler{
		encryptionProvider: provider,
		keyHandlers: []keys.Handler{
			staticHandler(t, 0, "new-key"),
			recoveryHandler(t, 1, nil),
		},
	}

	_, err := h.Sync(t.Context(), zaptest.NewLogger(t), "/dev/null")
	require.Error(t, err)
	assert.Empty(t, provider.added)
	assert.Empty(t, provider.removed)
}
