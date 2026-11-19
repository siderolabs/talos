// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sandboxd_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/siderolabs/go-cmd/pkg/cmd/proc/reaper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/siderolabs/talos/internal/app/machined/pkg/sandboxd"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/events"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
)

// fakeSandboxdEnv makes the test binary act as a sandboxd which exits once ready.
const fakeSandboxdEnv = "SANDBOXD_FAKE"

func TestMain(m *testing.M) {
	if os.Getenv(fakeSandboxdEnv) != "" {
		os.NewFile(3, "ready").Write([]byte{1}) //nolint:errcheck

		os.Exit(1)
	}

	os.Exit(m.Run())
}

type sandboxRuntime struct {
	runtime.Runtime

	logging runtime.LoggingManager
}

func (r *sandboxRuntime) Logging() runtime.LoggingManager { return r.logging }

func (r *sandboxRuntime) SetSandbox(runtime.SandboxLauncher) {}

// newRunner returns a runner launching path as sandboxd, and the directory it logs to.
func newRunner(t *testing.T, path string) (runner.Runner, string) {
	t.Helper()

	if os.Geteuid() != 0 {
		t.Skip("requires root to create the sandbox cgroup and namespaces")
	}

	// The runner waits for sandboxd through the reaper, as in machined.
	reaper.Run()
	t.Cleanup(reaper.Shutdown)

	logDir := t.TempDir()

	return sandboxd.NewRunner(&sandboxRuntime{logging: logging.NewFileLoggingManager(logDir)}, sandboxd.WithPath(path)), logDir
}

func eventSink(t *testing.T) events.Recorder {
	return func(state events.ServiceState, message string, args ...any) {
		t.Logf("state %s: %s", state, fmt.Sprintf(message, args...))
	}
}

// TestRunnerExitBeforeReady runs a sandboxd which exits before it is ready. When the reaper reaps
// it before the runner looks it up, the runner gets a PID-based process handle, which it kills while
// the waiter releases it.
//
// The reaper wins only sometimes, so run it many times.
func TestRunnerExitBeforeReady(t *testing.T) {
	r, _ := newRunner(t, "/bin/true")

	for range 200 {
		_, err := r.Run(t.Context(), eventSink(t), nil)
		require.ErrorContains(t, err, "sandboxd failed to start")
	}
}

// TestRunnerExitAfterReady runs a sandboxd which exits once it is ready, and checks the runner
// reports the PID it started in the exit log.
func TestRunnerExitAfterReady(t *testing.T) {
	self, err := os.Executable()
	require.NoError(t, err)

	r, logDir := newRunner(t, self)

	t.Setenv(fakeSandboxdEnv, "1")

	var pid int32

	_, err = r.Run(t.Context(), eventSink(t), func(p int32) { pid = p })
	require.ErrorContains(t, err, "sandboxd exited, namespace lost")
	require.Positive(t, pid)

	assert.EventuallyWithT(t, func(collect *assert.CollectT) {
		log, err := os.ReadFile(filepath.Join(logDir, sandboxd.ServiceID+".log"))
		require.NoError(collect, err)
		assert.Contains(collect, string(log), fmt.Sprintf("sandboxd (pid %d) exited", pid))
	}, 5*time.Second, 10*time.Millisecond)
}
