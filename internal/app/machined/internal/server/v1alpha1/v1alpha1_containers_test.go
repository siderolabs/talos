// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime //nolint:testpackage // to test the unexported kill after the stop timeout

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type fakeTask struct {
	containerdapi.Task

	pid     uint32
	killErr error
	killed  []syscall.Signal
}

func (t *fakeTask) Pid() uint32 {
	return t.pid
}

func (t *fakeTask) Kill(_ context.Context, signal syscall.Signal, _ ...containerdapi.KillOpts) error {
	t.killed = append(t.killed, signal)

	return t.killErr
}

type fakeContainer struct {
	containerdapi.Container

	task *fakeTask
}

func (c *fakeContainer) Task(context.Context, cio.Attach) (containerdapi.Task, error) {
	if c.task == nil {
		return nil, errdefs.ErrNotFound
	}

	return c.task, nil
}

func TestKillAfterStopTimeout(t *testing.T) {
	t.Parallel()

	const pid = 42

	for _, test := range []struct {
		name       string
		exited     bool
		waitFailed bool
		currentPID uint32
		gone       bool
		killed     bool
	}{
		{name: "running", currentPID: pid, killed: true},
		{name: "exited", exited: true, currentPID: pid},
		{name: "wait failed", waitFailed: true, currentPID: pid, killed: true},
		{name: "replaced", currentPID: pid + 1},
		{name: "gone", gone: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			task := &fakeTask{pid: test.currentPID}
			container := &fakeContainer{task: task}

			if test.gone {
				container.task = nil
			}

			statusC := make(chan containerdapi.ExitStatus, 1)

			switch {
			case test.exited:
				statusC <- containerdapi.ExitStatus{}
			case test.waitFailed:
				statusC <- *containerdapi.NewExitStatus(containerdapi.UnknownExitStatus, time.Time{}, errors.New("containerd is gone"))
			}

			start := time.Now()

			killAfterStopTimeout(t.Context(), zap.NewNop(), container, pid, statusC, 100*time.Millisecond)

			if test.killed {
				assert.Equal(t, []syscall.Signal{syscall.SIGKILL}, task.killed)
				assert.GreaterOrEqual(t, time.Since(start), 100*time.Millisecond)
			} else {
				assert.Empty(t, task.killed)
			}
		})
	}
}
