// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package yamlstrip_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/yamlstrip"
)

func TestComments(t *testing.T) {
	testCases, err := filepath.Glob(filepath.Join("testdata", "*.in.yaml"))
	require.NoError(t, err)

	for _, path := range testCases {
		t.Run(filepath.Base(path), func(t *testing.T) {
			in, err := os.ReadFile(path)
			require.NoError(t, err)

			expected, err := os.ReadFile(strings.ReplaceAll(path, ".in.yaml", ".out.yaml"))
			require.NoError(t, err)

			out := yamlstrip.Comments(in)
			require.Equal(t, string(expected), string(out))
		})
	}
}
