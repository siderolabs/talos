// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package imager

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	installerexitcode "github.com/siderolabs/talos/pkg/installer/exitcode"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestExecuteInvalidProfileStdin(t *testing.T) {
	oldStdin := os.Stdin
	oldOutputPath := cmdFlags.OutputPath

	r, w, err := os.Pipe()
	require.NoError(t, err)

	_, err = w.WriteString("not: [valid\n")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	os.Stdin = r
	cmdFlags.OutputPath = t.TempDir()

	rootCmd.SetArgs([]string{"-"})

	t.Cleanup(func() {
		os.Stdin = oldStdin
		cmdFlags.OutputPath = oldOutputPath

		rootCmd.SetArgs(nil)

		require.NoError(t, r.Close())
	})

	err = execute()
	require.Error(t, err)
	require.Equal(t, constants.ExitInvalidInput, installerexitcode.Resolve(err))
}
