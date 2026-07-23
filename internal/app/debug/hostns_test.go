// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package debug_test

import (
	"os/exec"
	"strconv"
	"testing"

	"github.com/siderolabs/go-cmd/pkg/cmd/proc/reaper"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/debug"
)

func TestWaitHostNsCommandWithReaper(t *testing.T) {
	reaper.Run()
	t.Cleanup(reaper.Shutdown)

	for _, expectedExitCode := range []int{0, 42} {
		t.Run(strconv.Itoa(expectedExitCode), func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "/bin/sh", "-c", "exit "+strconv.Itoa(expectedExitCode))
			notifyCh := make(chan reaper.ProcessInfo, 8)

			usingReaper := reaper.Notify(notifyCh)
			require.True(t, usingReaper)
			t.Cleanup(func() { reaper.Stop(notifyCh) })

			require.NoError(t, cmd.Start())

			exitCode, err := debug.WaitHostNsCommand(cmd, usingReaper, notifyCh)

			require.NoError(t, err)
			require.Equal(t, expectedExitCode, exitCode)
		})
	}
}
