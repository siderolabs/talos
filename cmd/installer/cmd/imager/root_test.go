// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package imager

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/profile"
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

func TestApplySDBootEnrollKeys(t *testing.T) {
	t.Run("empty is a no-op", func(t *testing.T) {
		var output profile.Output

		require.NoError(t, applySDBootEnrollKeys("", &output))
		require.Nil(t, output.ImageOptions)
		require.Nil(t, output.ISOOptions)
	})

	t.Run("force sets both image and ISO options", func(t *testing.T) {
		var output profile.Output

		require.NoError(t, applySDBootEnrollKeys("force", &output))
		require.NotNil(t, output.ImageOptions)
		require.Equal(t, profile.SDBootEnrollKeysForce, output.ImageOptions.SDBootEnrollKeys)
		require.NotNil(t, output.ISOOptions)
		require.Equal(t, profile.SDBootEnrollKeysForce, output.ISOOptions.SDBootEnrollKeys)
	})

	t.Run("preserves existing image options", func(t *testing.T) {
		output := profile.Output{
			ImageOptions: &profile.ImageOptions{
				DiskSize:   1234,
				DiskFormat: profile.DiskFormatRaw,
			},
		}

		require.NoError(t, applySDBootEnrollKeys("force", &output))
		require.Equal(t, int64(1234), output.ImageOptions.DiskSize)
		require.Equal(t, profile.DiskFormatRaw, output.ImageOptions.DiskFormat)
		require.Equal(t, profile.SDBootEnrollKeysForce, output.ImageOptions.SDBootEnrollKeys)
	})

	t.Run("invalid value is rejected as invalid input", func(t *testing.T) {
		var output profile.Output

		err := applySDBootEnrollKeys("bogus", &output)
		require.Error(t, err)
		require.Equal(t, constants.ExitInvalidInput, installerexitcode.Resolve(err))
	})
}
