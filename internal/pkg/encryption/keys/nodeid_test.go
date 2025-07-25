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
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

func TestNodeID(t *testing.T) {
	t.Parallel()

	handler := keys.NewNodeIDKeyHandler(keys.KeyHandler{}, "test", func(context.Context) (*hardware.SystemInformation, error) {
		res := hardware.NewSystemInformation(hardware.SystemInformationID)
		res.TypedSpec().UUID = "12345678-1234-5678-1234-567812345678"

		return res, nil
	})

	key, token, err := handler.NewKey(t.Context())
	require.NoError(t, err)
	require.Nil(t, token)

	assert.Equal(t, "12345678-1234-5678-1234-567812345678test", string(key.Value))

	key, err = handler.GetKey(t.Context(), nil)
	require.NoError(t, err)
	assert.Equal(t, "12345678-1234-5678-1234-567812345678test", string(key.Value))
}

func TestNodeIDBadEntropy(t *testing.T) {
	t.Parallel()

	handler := keys.NewNodeIDKeyHandler(keys.KeyHandler{}, "test", func(context.Context) (*hardware.SystemInformation, error) {
		res := hardware.NewSystemInformation(hardware.SystemInformationID)
		res.TypedSpec().UUID = "11111111-0000-1111-1111-111111111111" // bad entropy

		return res, nil
	})

	_, _, err := handler.NewKey(t.Context())
	require.Error(t, err)
	assert.EqualError(t, err, "machine UUID 11111111-0000-1111-1111-111111111111 entropy check failed")
}
