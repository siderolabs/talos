// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func stopProcessByPidfile(pidPath string) error {
	pidFile, err := os.Open(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
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

	if _, err = proc.Wait(); err != nil {
		if errors.Is(err, syscall.ECHILD) {
			return nil
		}

		return fmt.Errorf("error waiting for %d to exit (path %q): %w", pid, pidPath, err)
	}

	return nil
}
