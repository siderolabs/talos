// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/siderolabs/go-retry/retry"
)

// StopProcessByPidfile stops a process by reading its PID from a file.
func StopProcessByPidfile(pidPath string) error {
	pidFile, err := os.Open(pidPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("error checking PID file %q: %w", pidPath, err)
	}

	defer pidFile.Close() //nolint:errcheck

	var pid int

	if _, err = fmt.Fscanf(pidFile, "%d", &pid); err != nil {
		return fmt.Errorf("error reading PID for %q: %w", pidPath, err)
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("error finding process %d for %q: %w", pid, pidPath, err)
	}

	if err = proc.Signal(syscall.SIGTERM); err != nil {
		if err.Error() == "os: process already finished" {
			return nil
		}

		return fmt.Errorf("error sending SIGTERM to %d (path %q): %w", pid, pidPath, err)
	}

	// wait for the process to exit, this is using (unreliable and slow) polling
	return retry.Constant(30*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		// wait for the process if it's our child, we should clean up zombies, but if it's not our child, it would return ECHILD
		proc.Wait() //nolint:errcheck

		signalErr := proc.Signal(syscall.Signal(0))
		if signalErr == nil {
			return retry.ExpectedErrorf("process %d still running", pid)
		}

		return nil
	})
}
