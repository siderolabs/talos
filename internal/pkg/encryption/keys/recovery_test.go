// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys_test

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/siderolabs/go-blockdevice/v2/encryption/luks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/encryption/keys"
)

func TestRecoveryNewKey(t *testing.T) {
	t.Parallel()

	t.Run("no publisher", func(t *testing.T) {
		t.Parallel()

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", nil, nil)

		_, _, err := handler.NewKey(t.Context())
		require.ErrorIs(t, err, keys.ErrKeyNotAvailable)
	})

	t.Run("generates and publishes", func(t *testing.T) {
		t.Parallel()

		var (
			publishedVolume string
			publishedKey    []byte
		)

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", nil, func(_ context.Context, volumeID string, key []byte) error {
			publishedVolume = volumeID
			publishedKey = key

			return nil
		})

		key, tok, err := handler.NewKey(t.Context())
		require.NoError(t, err)

		assert.Equal(t, "STATE", publishedVolume)
		assert.Equal(t, publishedKey, key.Value)

		// 32 random bytes, base64-encoded
		raw, err := base64.StdEncoding.DecodeString(string(key.Value))
		require.NoError(t, err)
		assert.Len(t, raw, 32)

		recoveryToken, ok := tok.(*luks.Token[*keys.RecoveryToken])
		require.True(t, ok)
		assert.Equal(t, keys.TokenTypeRecovery, recoveryToken.Type)
		assert.False(t, recoveryToken.UserData.Fetched)
	})

	t.Run("publish failure", func(t *testing.T) {
		t.Parallel()

		publishErr := errors.New("boom")

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", nil, func(context.Context, string, []byte) error {
			return publishErr
		})

		_, _, err := handler.NewKey(t.Context())
		require.ErrorIs(t, err, publishErr)
	})
}

func TestRecoveryGetKey(t *testing.T) {
	t.Parallel()

	const secret = "correct horse battery staple"

	supplied := func(key []byte) func(context.Context, string) ([]byte, bool, error) {
		return func(context.Context, string) ([]byte, bool, error) {
			return key, key != nil, nil
		}
	}

	fetchedToken := &luks.Token[*keys.RecoveryToken]{Type: keys.TokenTypeRecovery, UserData: &keys.RecoveryToken{Fetched: true}}
	unfetchedToken := &luks.Token[*keys.RecoveryToken]{Type: keys.TokenTypeRecovery, UserData: &keys.RecoveryToken{Fetched: false}}

	t.Run("no getter", func(t *testing.T) {
		t.Parallel()

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", nil, nil)

		_, err := handler.GetKey(t.Context(), fetchedToken)
		require.ErrorIs(t, err, keys.ErrKeyNotAvailable)
	})

	t.Run("key not supplied, fetched", func(t *testing.T) {
		t.Parallel()

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", supplied(nil), nil)

		_, err := handler.GetKey(t.Context(), fetchedToken)
		require.ErrorIs(t, err, keys.ErrKeyNotAvailable)
	})

	t.Run("key not supplied, never fetched", func(t *testing.T) {
		t.Parallel()

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", supplied(nil), nil)

		_, err := handler.GetKey(t.Context(), unfetchedToken)
		require.ErrorIs(t, err, keys.ErrTokenInvalid)
	})

	t.Run("key supplied", func(t *testing.T) {
		t.Parallel()

		var requestedVolume string

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", func(_ context.Context, volumeID string) ([]byte, bool, error) {
			requestedVolume = volumeID

			return []byte(secret), true, nil
		}, nil)

		key, err := handler.GetKey(t.Context(), unfetchedToken)
		require.NoError(t, err)
		assert.Equal(t, secret, string(key.Value))
		assert.Equal(t, "STATE", requestedVolume)
	})

	t.Run("getter error", func(t *testing.T) {
		t.Parallel()

		getterErr := errors.New("boom")

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", func(context.Context, string) ([]byte, bool, error) {
			return nil, false, getterErr
		}, nil)

		_, err := handler.GetKey(t.Context(), fetchedToken)
		require.ErrorIs(t, err, getterErr)
		require.NotErrorIs(t, err, keys.ErrKeyNotAvailable)
	})
}
