// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMarkFlagDeprecated asserts that the warning about a deprecated flag stays off the command
// output stream: cobra prints its own flag deprecation warnings there, and for talosctl that
// stream is stdout, where they would be mixed into the output of the command.
func TestMarkFlagDeprecated(t *testing.T) {
	registered := len(deprecatedFlags)

	t.Cleanup(func() { deprecatedFlags = deprecatedFlags[:registered] })

	var out, stderr bytes.Buffer

	cmd := &cobra.Command{Use: "test", Run: func(*cobra.Command, []string) {}}
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.Flags().BoolP("kubernetes", "k", false, "use the k8s.io containerd namespace")

	require.NoError(t, MarkFlagDeprecated(cmd.Flags(), "kubernetes", "use --namespace cri instead"))

	assert.True(t, cmd.Flags().Lookup("kubernetes").Hidden)

	warnOnDeprecatedFlags(&stderr)
	assert.Empty(t, stderr.String(), "the flag was not used")

	cmd.SetArgs([]string{"-k"})
	require.NoError(t, cmd.Execute())

	assert.Empty(t, out.String(), "cobra should not print the warning itself")

	warnOnDeprecatedFlags(&stderr)
	assert.Equal(t, "Flag --kubernetes has been deprecated, use --namespace cri instead\n", stderr.String())
}

func TestMarkFlagDeprecatedErrors(t *testing.T) {
	registered := len(deprecatedFlags)

	t.Cleanup(func() { deprecatedFlags = deprecatedFlags[:registered] })

	flags := &cobra.Command{Use: "test"}

	flags.Flags().Bool("flag", false, "")

	assert.Error(t, MarkFlagDeprecated(flags.Flags(), "missing", "gone"))
	assert.Error(t, MarkFlagDeprecated(flags.Flags(), "flag", ""))
}
