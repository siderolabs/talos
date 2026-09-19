// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/encryption/keys"
)

func TestRecovery(t *testing.T) {
	t.Parallel()

	t.Run("no getter", func(t *testing.T) {
		t.Parallel()

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", nil)

		_, _, err := handler.NewKey(t.Context())
		require.ErrorIs(t, err, keys.ErrKeyNotAvailable)

		_, err = handler.GetKey(t.Context(), nil)
		require.ErrorIs(t, err, keys.ErrKeyNotAvailable)
	})

	t.Run("key not supplied", func(t *testing.T) {
		t.Parallel()

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", func(context.Context, string) ([]byte, bool, error) {
			return nil, false, nil
		})

		_, _, err := handler.NewKey(t.Context())
		require.ErrorIs(t, err, keys.ErrKeyNotAvailable)

		_, err = handler.GetKey(t.Context(), nil)
		require.ErrorIs(t, err, keys.ErrKeyNotAvailable)
	})

	t.Run("key supplied", func(t *testing.T) {
		t.Parallel()

		const secret = "correct horse battery staple"

		var requestedVolume string

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", func(_ context.Context, volumeID string) ([]byte, bool, error) {
			requestedVolume = volumeID

			return []byte(secret), true, nil
		})

		key, token, err := handler.NewKey(t.Context())
		require.NoError(t, err)
		require.Nil(t, token)

		assert.Equal(t, secret, string(key.Value))
		assert.Equal(t, "STATE", requestedVolume)

		key, err = handler.GetKey(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, secret, string(key.Value))
	})

	t.Run("getter error", func(t *testing.T) {
		t.Parallel()

		getterErr := errors.New("boom")

		handler := keys.NewRecoveryKeyHandler(keys.KeyHandler{}, "STATE", func(context.Context, string) ([]byte, bool, error) {
			return nil, false, getterErr
		})

		_, err := handler.GetKey(t.Context(), nil)
		require.ErrorIs(t, err, getterErr)
		require.NotErrorIs(t, err, keys.ErrKeyNotAvailable)
	})
}
