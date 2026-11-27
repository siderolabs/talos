// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd //nolint:testpackage // test the unexported wait result handling

import (
	"context"
	"syscall"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRetryTaskWait(t *testing.T) {
	t.Parallel()

	unavailableErr := status.Error(codes.Unavailable, "containerd is restarting")
	permissionErr := status.Error(codes.PermissionDenied, "wait denied")

	for _, test := range []struct {
		name     string
		status   *containerd.ExitStatus
		ok       bool
		retry    bool
		expected error
	}{
		{
			name:   "exit",
			status: containerd.NewExitStatus(0, time.Unix(1, 0), nil),
			ok:     true,
		},
		{
			name:   "containerd unavailable",
			status: containerd.NewExitStatus(containerd.UnknownExitStatus, time.Time{}, unavailableErr),
			ok:     true,
			retry:  true,
		},
		{
			name:     "non-transient error",
			status:   containerd.NewExitStatus(containerd.UnknownExitStatus, time.Time{}, permissionErr),
			ok:       true,
			expected: permissionErr,
		},
		{
			name:     "closed wait channel",
			status:   containerd.NewExitStatus(0, time.Time{}, nil),
			expected: errTaskWaitClosed,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			retry, err := retryTaskWait(*test.status, test.ok)
			require.Equal(t, test.retry, retry)
			require.ErrorIs(t, err, test.expected)
		})
	}
}

type fakeTask struct {
	containerd.Task

	waits     []func() (<-chan containerd.ExitStatus, error)
	kills     []error
	blockKill bool
}

func (t *fakeTask) ID() string {
	return "fake"
}

func (t *fakeTask) Wait(context.Context) (<-chan containerd.ExitStatus, error) {
	wait := t.waits[0]

	if len(t.waits) > 1 {
		t.waits = t.waits[1:]
	}

	return wait()
}

func (t *fakeTask) Kill(ctx context.Context, _ syscall.Signal, _ ...containerd.KillOpts) error {
	if t.blockKill {
		<-ctx.Done()

		return ctx.Err()
	}

	err := t.kills[0]

	if len(t.kills) > 1 {
		t.kills = t.kills[1:]
	}

	return err
}

func exitWith(exitStatus *containerd.ExitStatus) func() (<-chan containerd.ExitStatus, error) {
	return func() (<-chan containerd.ExitStatus, error) {
		ch := make(chan containerd.ExitStatus, 1)
		ch <- *exitStatus

		return ch, nil
	}
}

func running() (<-chan containerd.ExitStatus, error) {
	return make(chan containerd.ExitStatus), nil
}

func TestWaitTaskExit(t *testing.T) {
	t.Parallel()

	unavailableErr := status.Error(codes.Unavailable, "containerd is restarting")
	permissionErr := status.Error(codes.PermissionDenied, "wait denied")

	for _, test := range []struct {
		name     string
		waits    []func() (<-chan containerd.ExitStatus, error)
		exited   bool
		expected error
	}{
		{
			name:   "exit",
			waits:  []func() (<-chan containerd.ExitStatus, error){exitWith(containerd.NewExitStatus(0, time.Unix(1, 0), nil))},
			exited: true,
		},
		{
			name: "wait stream interrupted by a containerd restart, then exit",
			waits: []func() (<-chan containerd.ExitStatus, error){
				exitWith(containerd.NewExitStatus(containerd.UnknownExitStatus, time.Time{}, unavailableErr)),
				func() (<-chan containerd.ExitStatus, error) { return nil, unavailableErr },
				exitWith(containerd.NewExitStatus(0, time.Unix(1, 0), nil)),
			},
			exited: true,
		},
		{
			name: "wait stream interrupted by a containerd restart, task still running",
			waits: []func() (<-chan containerd.ExitStatus, error){
				exitWith(containerd.NewExitStatus(containerd.UnknownExitStatus, time.Time{}, unavailableErr)),
				running,
			},
		},
		{
			name:  "task still running",
			waits: []func() (<-chan containerd.ExitStatus, error){running},
		},
		{
			name: "task already gone",
			waits: []func() (<-chan containerd.ExitStatus, error){
				exitWith(containerd.NewExitStatus(containerd.UnknownExitStatus, time.Time{}, status.Error(codes.NotFound, "task not found"))),
			},
			exited: true,
		},
		{
			name:     "non-transient error",
			waits:    []func() (<-chan containerd.ExitStatus, error){func() (<-chan containerd.ExitStatus, error) { return nil, permissionErr }},
			expected: permissionErr,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			exited, err := waitTaskExit(t.Context(), &fakeTask{waits: test.waits}, time.Now().Add(3*taskWaitRetryInterval))
			require.Equal(t, test.exited, exited)
			require.ErrorIs(t, err, test.expected)
		})
	}
}

func TestKillTask(t *testing.T) {
	t.Parallel()

	unavailableErr := status.Error(codes.Unavailable, "containerd is restarting")
	permissionErr := status.Error(codes.PermissionDenied, "kill denied")

	for _, test := range []struct {
		name      string
		kills     []error
		blockKill bool
		expected  error
	}{
		{
			name:  "killed",
			kills: []error{nil},
		},
		{
			name:  "already gone",
			kills: []error{errdefs.ErrNotFound},
		},
		{
			name:  "already gone, grpc error",
			kills: []error{status.Error(codes.NotFound, "task not found")},
		},
		{
			name:  "containerd restarting, then killed",
			kills: []error{unavailableErr, nil},
		},
		{
			name:     "containerd unavailable until the deadline",
			kills:    []error{unavailableErr},
			expected: unavailableErr,
		},
		{
			name:     "non-transient error",
			kills:    []error{permissionErr},
			expected: permissionErr,
		},
		{
			name:      "kill call hanging past the deadline",
			blockKill: true,
			expected:  context.DeadlineExceeded,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := killTask(t.Context(), &fakeTask{kills: test.kills, blockKill: test.blockKill}, syscall.SIGTERM, time.Now().Add(3*taskWaitRetryInterval))
			require.ErrorIs(t, err, test.expected)
		})
	}
}
