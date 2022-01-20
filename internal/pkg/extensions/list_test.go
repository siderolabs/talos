// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/internal/pkg/extensions"
)

func TestList(t *testing.T) {
	extensions, err := extensions.List("testdata/good/")
	require.NoError(t, err)

	require.Len(t, extensions, 1)

	assert.Equal(t, "gvisor", extensions[0].Manifest.Metadata.Name)
}
