// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/encryption/keys"
)

func TestStatic(t *testing.T) {
	t.Parallel()

	const secret = "topsecret"

	handler := keys.NewStaticKeyHandler(keys.KeyHandler{}, []byte(secret))

	key, token, err := handler.NewKey(t.Context())
	require.NoError(t, err)
	require.Nil(t, token)

	assert.Equal(t, secret, string(key.Value))

	key1, err := handler.GetKey(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, key, key1)
}
