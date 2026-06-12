// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
)

func TestEncryptionSpecAllowDiscards(t *testing.T) {
	t.Parallel()

	// defaults to false when unset
	assert.False(t, block.EncryptionSpec{}.AllowDiscards())

	assert.True(t, block.EncryptionSpec{EncryptionAllowDiscards: new(true)}.AllowDiscards())
	assert.False(t, block.EncryptionSpec{EncryptionAllowDiscards: new(false)}.AllowDiscards())
}
