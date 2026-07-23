// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd //nolint:testpackage // test the unexported wait result handling

import (
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
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
