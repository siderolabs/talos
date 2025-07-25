// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/encryption/keys"
)

func TestSalted(t *testing.T) {
	t.Parallel()

	const (
		secret = "topsecret"
		salt   = "salted"
	)

	inner := keys.NewStaticKeyHandler(keys.KeyHandler{}, []byte(secret))

	handler := keys.NewSaltedHandler(inner, func(context.Context) ([]byte, error) {
		return []byte(salt), nil
	})

	key, token, err := handler.NewKey(t.Context())
	require.NoError(t, err)
	require.Nil(t, token)

	assert.Equal(t, secret+salt, string(key.Value))

	key, err = handler.GetKey(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, secret+salt, string(key.Value))
}
