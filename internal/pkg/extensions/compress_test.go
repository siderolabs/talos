// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/extensions"
)

func TestCopyFilesTruncatesExistingDestination(t *testing.T) {
	t.Parallel()

	sourcePath := filepath.Join(t.TempDir(), "modules.softdep")
	destinationPath := filepath.Join(t.TempDir(), "modules.softdep")

	require.NoError(t, os.WriteFile(sourcePath, []byte("short\n"), 0o644))
	require.NoError(t, os.WriteFile(destinationPath, []byte("long destination content\n"), 0o644))
	require.NoError(t, extensions.CopyFiles(sourcePath, destinationPath))

	contents, err := os.ReadFile(destinationPath)
	require.NoError(t, err)
	require.Equal(t, "short\n", string(contents))
}
