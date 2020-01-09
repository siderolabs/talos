// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package limits

import (
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// FileLimitTask represents the FileLimitTask task.
type FileLimitTask struct{}

// NewFileLimitTask initializes and returns a FileLimitTask task.
func NewFileLimitTask() phase.Task {
	return &FileLimitTask{}
}

// TaskFunc returns the runtime function.
func (task *FileLimitTask) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *FileLimitTask) standard(r runtime.Runtime) (err error) {
	// TODO(andrewrynhard): Should we read limit from /proc/sys/fs/nr_open?
	if err = unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{Cur: 1048576, Max: 1048576}); err != nil {
		return err
	}

	return nil
}
