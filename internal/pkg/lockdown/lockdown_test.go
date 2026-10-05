// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lockdown_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/lockdown"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestReadState(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		contents string

		expected    runtimeres.LockdownState
		expectError bool
	}{
		{
			name:     "none",
			contents: "[none] integrity confidentiality\n",
			expected: runtimeres.LockdownStateNone,
		},
		{
			name:     "integrity",
			contents: "none [integrity] confidentiality\n",
			expected: runtimeres.LockdownStateIntegrity,
		},
		{
			name:     "confidentiality",
			contents: "none integrity [confidentiality]\n",
			expected: runtimeres.LockdownStateConfidentiality,
		},
		{
			name:        "no active level",
			contents:    "none integrity confidentiality\n",
			expectError: true,
		},
		{
			name:        "unknown level",
			contents:    "[paranoid] none\n",
			expectError: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "lockdown")
			require.NoError(t, os.WriteFile(path, []byte(test.contents), 0o644))

			state, err := lockdown.ReadState(path)
			if test.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, state)
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()

		_, err := lockdown.ReadState(filepath.Join(t.TempDir(), "missing"))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}
