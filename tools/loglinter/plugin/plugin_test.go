// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package plugin_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/tools/loglinter/plugin"
)

func TestNewSupportsInlineConfig(t *testing.T) {
	root := t.TempDir()

	writePluginTestFile(t, root, ".golangci.yml", "version: \"2\"\n")

	t.Chdir(root)

	previousArgs := os.Args

	t.Cleanup(func() {
		os.Args = previousArgs
	})

	os.Args = []string{"custom-gcl", "run", "--config", ".golangci.yml"}

	linter, err := plugin.New(map[string]any{
		"rules": map[string]any{
			"stdlib_log_calls": map[string]any{
				"allow": []string{"allowed.go"},
			},
		},
	})
	require.NoError(t, err)

	analyzers, err := linter.BuildAnalyzers()
	require.NoError(t, err)
	require.Len(t, analyzers, 1)
	require.Equal(t, plugin.Name, analyzers[0].Name)
}

func writePluginTestFile(t *testing.T, root, relPath, content string) {
	t.Helper()

	path := filepath.Join(root, relPath)

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
