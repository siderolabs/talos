// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package version_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/version"
)

func TestOSRelease(t *testing.T) {
	t.Parallel()

	// simply verify generation without errors
	_, err := version.OSRelease()
	require.NoError(t, err)
}
