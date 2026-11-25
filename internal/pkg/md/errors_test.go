// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package md_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/md"
)

func TestErrorClassification(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		stderr string
		want   error
	}{
		{name: "exists", stderr: "appears to be part of a raid array", want: md.ErrExists},
		{name: "in use", stderr: "Device or resource busy", want: md.ErrInUse},
		{name: "not found", stderr: "cannot open /dev/md0: No such file or directory", want: md.ErrNotFound},
		{name: "invalid", stderr: "invalid option -- nope", want: md.ErrInvalidArgument},
		{name: "resync", stderr: "performing resync/recovery and cannot be reshaped", want: md.ErrResync},
		{name: "resync short", stderr: "md: resync or recovery in progress", want: md.ErrResync},
		{name: "other", stderr: "boom", want: md.ErrCommand},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stderrPath := filepath.Join(t.TempDir(), "stderr")
			require.NoError(t, os.WriteFile(stderrPath, []byte("mdadm: "+tt.stderr+"\n"), 0o644))

			m := newScriptMD(t, fmt.Sprintf("cat '%s' >&2\nexit 3\n", stderrPath))

			err := m.Stop(t.Context(), "/dev/md0")
			require.ErrorIs(t, err, tt.want)

			var execErr *md.ExecError

			require.ErrorAs(t, err, &execErr)
			assert.Equal(t, 3, execErr.ExitCode)
			assert.Equal(t, "mdadm: "+tt.stderr+"\n", string(execErr.Stderr))
		})
	}
}
