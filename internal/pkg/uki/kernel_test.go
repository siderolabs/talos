// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package uki_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/uki"
)

func TestKernelVersion(t *testing.T) {
	version, err := uki.DiscoverKernelVersion("testdata/kernel")
	require.NoError(t, err)

	assert.Equal(t, "6.1.58-talos", version)
}
