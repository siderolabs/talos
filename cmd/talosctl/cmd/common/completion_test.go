// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package common_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/common"
)

// setupPatchDir creates a directory with a fixed set of entries to complete against.
func setupPatchDir(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	for _, name := range []string{"custom-config.yaml", "custom-other.yml", "patch.json", "notes.txt", ".hidden.yaml"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), nil, 0o644))
	}

	require.NoError(t, os.Mkdir(filepath.Join(root, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "inner.yaml"), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "subnet.yaml"), nil, 0o644))
	require.NoError(t, os.Mkdir(filepath.Join(root, "secrets"), 0o755))

	return root
}

// Note: no t.Parallel, as t.Chdir is incompatible with it.
func TestCompleteConfigPatch(t *testing.T) {
	root := setupPatchDir(t)

	for _, test := range []struct {
		name       string
		toComplete string
		expected   []string
		directive  cobra.ShellCompDirective
	}{
		{
			name:       "prefix match",
			toComplete: "@cu",
			expected:   []string{"@custom-config.yaml", "@custom-other.yml"},
			directive:  cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:       "files and directories",
			toComplete: "@sub",
			expected:   []string{"@sub/", "@subnet.yaml"},
			directive:  cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace,
		},
		{
			name:       "dot slash prefix is preserved",
			toComplete: "@./cu",
			expected:   []string{"@./custom-config.yaml", "@./custom-other.yml"},
			directive:  cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:       "inside a directory",
			toComplete: "@sub/",
			expected:   []string{"@sub/inner.yaml"},
			directive:  cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:       "missing directory",
			toComplete: "@nope/",
			directive:  cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:       "hidden entries are skipped",
			toComplete: "@",
			expected: []string{
				"@notes.txt", "@patch.json", "@secrets/", "@custom-config.yaml", "@custom-other.yml", "@sub/", "@subnet.yaml",
			},
			directive: cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace,
		},
		{
			name:       "hidden entries when asked for",
			toComplete: "@.",
			expected:   []string{"@.hidden.yaml"},
			directive:  cobra.ShellCompDirectiveNoFileComp,
		},
		{
			name:       "empty input",
			toComplete: "",
			expected: []string{
				"@notes.txt", "@patch.json", "@secrets/", "@custom-config.yaml", "@custom-other.yml", "@sub/", "@subnet.yaml",
			},
			directive: cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace,
		},
		{
			name:       "bare filename is left to the shell",
			toComplete: "sma",
			directive:  cobra.ShellCompDirectiveDefault,
		},
		{
			name:       "inline patch is left to the shell",
			toComplete: "machine:\n  network: {}",
			directive:  cobra.ShellCompDirectiveDefault,
		},
		{
			name:       "tilde is not expanded",
			toComplete: "@~/",
			directive:  cobra.ShellCompDirectiveNoFileComp,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(root)

			completions, directive := common.CompleteConfigPatch(nil, nil, test.toComplete)

			assert.ElementsMatch(t, test.expected, completions)
			assert.Equal(t, test.directive, directive)
		})
	}
}

func TestCompleteConfigPatchAbsolutePath(t *testing.T) {
	root := setupPatchDir(t)

	completions, directive := common.CompleteConfigPatch(nil, nil, "@"+filepath.Join(root, "cu"))

	assert.ElementsMatch(t, []string{
		"@" + filepath.Join(root, "custom-config.yaml"),
		"@" + filepath.Join(root, "custom-other.yml"),
	}, completions)
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
}

// Note: no t.Parallel, as t.Chdir is incompatible with it.
func TestCompleteConfigPatchSymlinkedDir(t *testing.T) {
	root := setupPatchDir(t)

	require.NoError(t, os.Symlink(filepath.Join(root, "sub"), filepath.Join(root, "linked")))

	t.Chdir(root)

	completions, directive := common.CompleteConfigPatch(nil, nil, "@link")

	assert.Equal(t, []string{"@linked/"}, completions)
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp|cobra.ShellCompDirectiveNoSpace, directive)
}
