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
)

// IsProcessRunning checks if a process is running by reading its PID from a file
// and sending signal 0 to verify it's alive.
func IsProcessRunning(pidPath string) bool {
	pidFile, err := os.Open(pidPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false
		}

		return false
	}

	defer pidFile.Close() //nolint:errcheck

	var pid int

	if _, err = fmt.Fscanf(pidFile, "%d", &pid); err != nil {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Signal 0 doesn't send any signal but checks if the process exists
	err = proc.Signal(syscall.Signal(0))

	return err == nil
}
