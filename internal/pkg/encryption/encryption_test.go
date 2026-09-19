// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package encryption_test

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-blockdevice/v2/encryption/luks"
	"github.com/siderolabs/go-blockdevice/v2/encryption/token"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	talosencryption "github.com/siderolabs/talos/internal/pkg/encryption"
	"github.com/siderolabs/talos/internal/pkg/encryption/keys"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// fakeProvider is an in-memory encryption provider which tracks keyslots, tokens and the operations performed on them.
type fakeProvider struct {
	slots  map[int][]byte
	tokens map[int][]byte

	encrypted bool
	removed   []int
	added     []int
}

func newFakeProvider(slots map[int][]byte) *fakeProvider {
	return &fakeProvider{slots: slots, tokens: map[int][]byte{}}
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
	delete(p.tokens, slot)
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

func (p *fakeProvider) SetToken(_ context.Context, _ string, slot int, t token.Token) error {
	data, err := t.Bytes()
	if err != nil {
		return err
	}

	p.tokens[slot] = data

	return nil
}

func (p *fakeProvider) ReadToken(_ context.Context, _ string, slot int, t token.Token) error {
	data, ok := p.tokens[slot]
	if !ok {
		return encryption.ErrTokenNotFound
	}

	return t.Decode(data)
}

func (p *fakeProvider) RemoveToken(_ context.Context, _ string, slot int) error {
	delete(p.tokens, slot)

	return nil
}

// recoveryToken returns the recovery token stored for the slot, if any.
func (p *fakeProvider) recoveryToken(t *testing.T, slot int) *keys.RecoveryToken {
	t.Helper()

	data, ok := p.tokens[slot]
	if !ok {
		return nil
	}

	tok := &luks.Token[*keys.RecoveryToken]{}
	require.NoError(t, tok.Decode(data))

	return tok.UserData
}

// recoveryGetter returns a recovery key getter which returns the key only if it's set.
func recoveryGetter(key *[]byte) func(context.Context, string) ([]byte, bool, error) {
	return func(context.Context, string) ([]byte, bool, error) {
		if key == nil || *key == nil {
			return nil, false, nil
		}

		return *key, true, nil
	}
}

// recoveryPublisher returns a recovery key publisher which records the published key.
func recoveryPublisher(published *[]byte) func(context.Context, string, []byte) error {
	return func(_ context.Context, _ string, key []byte) error {
		*published = key

		return nil
	}
}

func staticHandler(t *testing.T, passphrase string) keys.Handler {
	t.Helper()

	handler, err := keys.NewHandler(block.EncryptionKey{
		Slot:             0,
		Type:             block.EncryptionKeyStatic,
		StaticPassphrase: []byte(passphrase),
	})
	require.NoError(t, err)

	return handler
}

func recoveryHandler(t *testing.T, supplied, published *[]byte, salt []byte) keys.Handler {
	t.Helper()

	opts := []keys.KeyOption{
		keys.WithVolumeID("STATE"),
		keys.WithRecoveryKeyGetter(recoveryGetter(supplied)),
		keys.WithSaltGetter(func(context.Context) ([]byte, error) {
			return salt, nil
		}),
	}

	if published != nil {
		opts = append(opts, keys.WithRecoveryKeyPublisher(recoveryPublisher(published)))
	}

	handler, err := keys.NewHandler(block.EncryptionKey{
		Slot:        1,
		Type:        block.EncryptionKeyRecovery,
		LockToSTATE: salt != nil,
	}, opts...)
	require.NoError(t, err)

	return handler
}

func TestRecoveryKeyFormatAndEncrypt(t *testing.T) {
	t.Parallel()

	t.Run("generated at format time", func(t *testing.T) {
		t.Parallel()

		var published []byte

		provider := newFakeProvider(map[int][]byte{})

		h := talosencryption.NewTestHandler(provider, []keys.Handler{
			staticHandler(t, "static-key"),
			recoveryHandler(t, nil, &published, nil),
		})

		require.NoError(t, h.FormatAndEncrypt(t.Context(), zaptest.NewLogger(t), "/dev/null"))

		assert.True(t, provider.encrypted)
		assert.Len(t, provider.slots, 2)
		assert.Equal(t, []int{1}, provider.added)
		assert.Equal(t, published, provider.slots[1])

		tok := provider.recoveryToken(t, 1)
		require.NotNil(t, tok)
		assert.False(t, tok.Fetched)
	})

	t.Run("no publisher", func(t *testing.T) {
		t.Parallel()

		provider := newFakeProvider(map[int][]byte{})

		h := talosencryption.NewTestHandler(provider, []keys.Handler{
			staticHandler(t, "static-key"),
			recoveryHandler(t, nil, nil, nil),
		})

		// the recovery slot can't be enrolled, the volume should still be formatted
		require.NoError(t, h.FormatAndEncrypt(t.Context(), zaptest.NewLogger(t), "/dev/null"))

		assert.True(t, provider.encrypted)
		assert.Len(t, provider.slots, 1)
		assert.Empty(t, provider.added)
	})
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

		enrolled    bool
		fetched     bool
		lockToState bool
		suppliedKey string

		expectRotated bool
		expectSlots   int
	}{
		{
			name: "pending slot is enrolled with a generated key",

			expectRotated: true, // added, not removed
			expectSlots:   2,
		},
		{
			name: "enrolled but never fetched key is regenerated",

			enrolled: true,

			expectRotated: true,
			expectSlots:   2,
		},
		{
			name: "fetched key is left alone",

			enrolled: true,
			fetched:  true,

			expectSlots: 2,
		},
		{
			name: "fetched key is left alone when the supplied key matches",

			enrolled:    true,
			fetched:     true,
			suppliedKey: recoveryKey,

			expectSlots: 2,
		},
		{
			name: "fetched key is left alone when the supplied key is wrong",

			enrolled:    true,
			fetched:     true,
			suppliedKey: "typo",

			expectSlots: 2,
		},
		{
			name: "fetched key locked to STATE is left alone when the supplied key is wrong",

			enrolled:    true,
			fetched:     true,
			lockToState: true,
			suppliedKey: "typo",

			expectSlots: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			provider := newFakeProvider(map[int][]byte{
				0: []byte(unlockKey),
			})

			var salt []byte

			if test.lockToState {
				salt = []byte("salt")
			}

			enrolledKey := slices.Concat([]byte(recoveryKey), salt)

			if test.enrolled {
				provider.slots[1] = enrolledKey

				require.NoError(t, provider.SetToken(t.Context(), "/dev/null", 1, &luks.Token[*keys.RecoveryToken]{
					Type:     keys.TokenTypeRecovery,
					UserData: &keys.RecoveryToken{KeySlots: []int{1}, Fetched: test.fetched},
				}))
			}

			var supplied, published []byte

			if test.suppliedKey != "" {
				supplied = []byte(test.suppliedKey)
			}

			h := talosencryption.NewTestHandler(provider, []keys.Handler{
				staticHandler(t, unlockKey),
				recoveryHandler(t, &supplied, &published, salt),
			})

			failedSyncs, err := h.SyncKeys(t.Context(), zaptest.NewLogger(t), "/dev/null", unlockedWith)
			require.NoError(t, err)

			// recovery key state is never a failed sync
			assert.Empty(t, failedSyncs)
			assert.Len(t, provider.slots, test.expectSlots)

			if test.expectRotated {
				assert.Equal(t, []int{1}, provider.added)
				assert.Equal(t, published, provider.slots[1], "the new key should be published")
				assert.NotEqual(t, []byte(recoveryKey), provider.slots[1])
				assert.False(t, provider.recoveryToken(t, 1).Fetched)
			} else {
				assert.Empty(t, provider.added)
				assert.Empty(t, provider.removed)
				assert.Nil(t, published)
				assert.Equal(t, enrolledKey, provider.slots[1])
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

	require.NoError(t, provider.SetToken(t.Context(), "/dev/null", 1, &luks.Token[*keys.RecoveryToken]{
		Type:     keys.TokenTypeRecovery,
		UserData: &keys.RecoveryToken{KeySlots: []int{1}, Fetched: true},
	}))

	var supplied, published []byte

	h := talosencryption.NewTestHandler(provider, []keys.Handler{
		staticHandler(t, "new-key"),
		recoveryHandler(t, &supplied, &published, nil),
	})

	// without the recovery key, the volume can't be opened
	_, _, failedSyncs, err := h.Open(t.Context(), zaptest.NewLogger(t), "/dev/null", "luks-STATE")
	require.Error(t, err)
	require.ErrorIs(t, err, keys.ErrKeyNotAvailable)
	assert.Empty(t, failedSyncs)

	// once the operator supplies the key, the volume opens with the recovery slot,
	// and the stale automatic slot is re-enrolled
	supplied = []byte(recoveryKey)

	path, usedSlot, failedSyncs, err := h.Open(t.Context(), zaptest.NewLogger(t), "/dev/null", "luks-STATE")
	require.NoError(t, err)

	assert.Equal(t, "/dev/mapper/luks-STATE", path)
	assert.Equal(t, 1, usedSlot)
	assert.Empty(t, failedSyncs)
	assert.Equal(t, []int{0}, provider.removed)
	assert.Equal(t, []int{0}, provider.added)
	assert.Equal(t, []byte("new-key"), provider.slots[0])
	assert.Nil(t, published, "the recovery key must not be regenerated")

	pending, err := h.PendingSlots("/dev/null")
	require.NoError(t, err)
	assert.Empty(t, pending)
}
