// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package encryption_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-blockdevice/v2/encryption/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	talosencryption "github.com/siderolabs/talos/internal/pkg/encryption"
	"github.com/siderolabs/talos/internal/pkg/encryption/keys"
)

// fakeHandler is a key handler which returns a fixed key or a fixed error.
type fakeHandler struct {
	slot   int
	key    []byte
	getErr error
}

func (h *fakeHandler) Slot() int { return h.slot }

func (h *fakeHandler) NewKey(context.Context) (*encryption.Key, token.Token, error) {
	return encryption.NewKey(h.slot, h.key), nil, nil
}

func (h *fakeHandler) GetKey(context.Context, token.Token) (*encryption.Key, error) {
	if h.getErr != nil {
		return nil, h.getErr
	}

	return encryption.NewKey(h.slot, h.key), nil
}

// fakeProvider is an in-memory encryption provider which tracks keyslots and the operations performed on them.
type fakeProvider struct {
	slots map[int][]byte

	removed []int
	added   []int
}

func newFakeProvider(slots map[int][]byte) *fakeProvider {
	return &fakeProvider{slots: slots}
}

func (p *fakeProvider) Encrypt(context.Context, string, *encryption.Key) error { return nil }

func (p *fakeProvider) IsOpen(context.Context, string, string) (bool, string, error) {
	return false, "", nil
}

func (p *fakeProvider) Open(context.Context, string, string, *encryption.Key) (string, error) {
	return "", nil
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

func TestSyncKeys(t *testing.T) {
	t.Parallel()

	const (
		unlockKey = "unlock-key"
		staleKey  = "stale-key"
	)

	// slot 0 is the key the volume was unlocked with, slot 1 is the key under test
	unlockedWith := encryption.NewKey(0, []byte(unlockKey))

	for _, test := range []struct {
		name string

		handler *fakeHandler

		expectedRemoved     []int
		expectedAdded       []int
		expectedFailedSyncs int
	}{
		{
			name: "valid key is left alone",

			handler: &fakeHandler{slot: 1, key: []byte(staleKey)},
		},
		{
			name: "changed key is re-enrolled",

			handler: &fakeHandler{slot: 1, key: []byte("new-key")},

			expectedRemoved: []int{1},
			expectedAdded:   []int{1},
		},
		{
			name: "invalid token is re-enrolled",

			handler: &fakeHandler{slot: 1, key: []byte("new-key"), getErr: keys.ErrTokenInvalid},

			expectedRemoved: []int{1},
			expectedAdded:   []int{1},
		},
		{
			name: "stale key is re-enrolled",

			handler: &fakeHandler{slot: 1, key: []byte("new-key"), getErr: fmt.Errorf("%w: sealing policy digest does not match", keys.ErrKeyStale)},

			expectedRemoved: []int{1},
			expectedAdded:   []int{1},
		},
		{
			name: "transient error is reported and slot is kept",

			handler: &fakeHandler{slot: 1, key: []byte("new-key"), getErr: errors.New("tpm is busy")},

			expectedFailedSyncs: 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			provider := newFakeProvider(map[int][]byte{
				0: []byte(unlockKey),
				1: []byte(staleKey),
			})

			h := talosencryption.NewTestHandler(provider, []keys.Handler{
				&fakeHandler{slot: 0, key: []byte(unlockKey)},
				test.handler,
			})

			failedSyncs, err := h.SyncKeys(t.Context(), zaptest.NewLogger(t), "/dev/null", unlockedWith)
			require.NoError(t, err)

			assert.Len(t, failedSyncs, test.expectedFailedSyncs)
			assert.Equal(t, test.expectedRemoved, provider.removed)
			assert.Equal(t, test.expectedAdded, provider.added)

			// slot 1 should always be present after the sync
			assert.Contains(t, provider.slots, 1)

			if len(test.expectedAdded) > 0 {
				assert.Equal(t, test.handler.key, provider.slots[1])
			}
		})
	}
}

func TestSyncKeysRemovesUndeclaredSlots(t *testing.T) {
	t.Parallel()

	unlockedWith := encryption.NewKey(0, []byte("unlock-key"))

	provider := newFakeProvider(map[int][]byte{
		0: []byte("unlock-key"),
		5: []byte("leftover"),
	})

	h := talosencryption.NewTestHandler(provider, []keys.Handler{
		&fakeHandler{slot: 0, key: []byte("unlock-key")},
	})

	failedSyncs, err := h.SyncKeys(t.Context(), zaptest.NewLogger(t), "/dev/null", unlockedWith)
	require.NoError(t, err)

	assert.Empty(t, failedSyncs)
	assert.Equal(t, []int{5}, provider.removed)
	assert.NotContains(t, provider.slots, 5)
}
