// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sandboxd_test

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/sandboxd"
)

func TestTerminationSignals(t *testing.T) {
	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT} {
		t.Run(sig.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTerminationSignalsHelper$")

			cmd.Env = append(os.Environ(), "SANDBOXD_SIGNAL_HELPER=1")
			cmd.Stderr = os.Stderr
			input, err := cmd.StdinPipe()
			require.NoError(t, err)
			output, err := cmd.StdoutPipe()
			require.NoError(t, err)
			require.NoError(t, cmd.Start())
			t.Cleanup(func() {
				if cmd.ProcessState == nil {
					cmd.Process.Kill() //nolint:errcheck // The helper may already have exited on failure.
					cmd.Wait()         //nolint:errcheck // A killed helper is expected to fail.
				}
			})

			lines := bufio.NewScanner(output)
			require.True(t, lines.Scan(), "helper must report child ignored mask: %v", lines.Err())
			mask, err := strconv.ParseUint(strings.TrimSpace(lines.Text()), 16, 64)
			require.NoError(t, err)

			for _, childSignal := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT} {
				assert.Zero(t, mask&(1<<uint(childSignal-1)), "child inherited ignored %s (mask %x)", childSignal, mask)
			}

			// READY is emitted only after Bash has installed its trap. Signal the
			// sandboxd helper separately to prove it survives (including SIGQUIT).
			require.True(t, lines.Scan())
			require.Equal(t, "READY", lines.Text())
			require.NoError(t, cmd.Process.Signal(sig))
			_, err = fmt.Fprintln(input, int(sig))
			require.NoError(t, err)
			require.True(t, lines.Scan())
			require.Equal(t, "CAUGHT", lines.Text())
			require.NoError(t, cmd.Wait())
		})
	}
}

func TestTerminationSignalsHelper(t *testing.T) {
	if os.Getenv("SANDBOXD_SIGNAL_HELPER") != "1" {
		return
	}

	stop := sandboxd.IgnoreTerminationSignals()
	defer stop()

	bash, err := exec.LookPath("bash")
	require.NoError(t, err)

	// Launch directly, not as a shell background job: Bash deliberately ignores
	// INT/QUIT for asynchronous jobs. Check the mask after installing traps:
	// Bash itself initially ignores QUIT, but cannot trap inherited SIG_IGN.
	const script = `trap 'printf "CAUGHT\n"' TERM INT HUP QUIT
 while read -r key value rest; do
    if [[ "$key" == SigIgn: ]]; then printf '%s\n' "$value"; fi
 done < /proc/self/status
 printf 'READY\n'
 read -r sig || exit 1
 kill -s "$sig" "$$"
 printf 'DONE\n'
 `

	launcher := cap.NewLauncher(bash, []string{bash, "--noprofile", "--norc", "-c", script}, os.Environ())
	launcher.Callback(func(attr *syscall.ProcAttr, _ any) error {
		attr.Files = []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()}

		return nil
	})
	pid, err := launcher.Launch(nil)
	require.NoError(t, err)
	child, err := os.FindProcess(pid)
	require.NoError(t, err)
	state, err := child.Wait()
	require.NoError(t, err)
	require.True(t, state.Success(), "Bash: %s", state)
}
