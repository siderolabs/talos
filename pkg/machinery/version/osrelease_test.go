// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package version_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/version"
)

func TestOSRelease(t *testing.T) {
	t.Parallel()

	// simply verify generation without errors
	_, err := version.OSRelease()
	require.NoError(t, err)
}

func TestOSReleaseFor(t *testing.T) {
	t.Parallel()

	contents, err := version.OSReleaseFor("Talos", "v1.0.0")
	require.NoError(t, err)

	assert.Equal(
		t,
		"NAME=\"Talos\"\nID=talos\nVERSION_ID=v1.0.0\nPRETTY_NAME=\"Talos (v1.0.0)\"\nHOME_URL=\"https://www.talos.dev/\"\nBUG_REPORT_URL=\"https://github.com/siderolabs/talos/issues\"\nVENDOR_NAME=\"Sidero Labs\"\nVENDOR_URL=\"https://www.siderolabs.com/\"\n", //nolint:lll
		string(contents),
	)
}
