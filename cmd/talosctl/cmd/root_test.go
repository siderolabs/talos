// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd //nolint:testpackage // to add a command to the unexported rootCmd

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/common"
)

// TestExecuteExitError covers a command passing on the exit code of a command run on the node: it is
// not printed as an error.
//
//nolint:paralleltest // replaces os.Stderr and the arguments of rootCmd
func TestExecuteExitError(t *testing.T) {
	exitCmd := &cobra.Command{
		Use:    "exit-error-test",
		Hidden: true,
		RunE: func(*cobra.Command, []string) error {
			return &common.ExitError{Code: 7}
		},
	}

	rootCmd.AddCommand(exitCmd)
	t.Cleanup(func() { rootCmd.RemoveCommand(exitCmd) })

	rootCmd.SetArgs([]string{"exit-error-test"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	stderrR, stderrW, err := os.Pipe()
	require.NoError(t, err)

	stderr := os.Stderr
	os.Stderr = stderrW

	t.Cleanup(func() { os.Stderr = stderr })

	err = Execute()

	os.Stderr = stderr

	require.NoError(t, stderrW.Close())

	printed, readErr := io.ReadAll(stderrR)
	require.NoError(t, readErr)

	exitErr, ok := errors.AsType[*common.ExitError](err)
	require.True(t, ok, "unexpected error: %v", err)
	assert.Equal(t, 7, exitErr.Code)
	assert.Empty(t, string(printed))
}
