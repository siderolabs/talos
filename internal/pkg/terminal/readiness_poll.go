// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !darwin && !windows

package terminal

import (
	"context"
	"errors"
	"fmt"
	"math"

	"golang.org/x/sys/unix"
)

const inputWaitMilliseconds = 100

func waitReadable(ctx context.Context, fd uintptr) error {
	if fd > math.MaxInt32 {
		return errors.New("terminal input file descriptor is invalid")
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN | unix.POLLHUP}}

		_, err := unix.Poll(fds, inputWaitMilliseconds)
		if errors.Is(err, unix.EINTR) {
			continue
		}

		if err != nil {
			return fmt.Errorf("poll terminal input: %w", err)
		}

		if fds[0].Revents&unix.POLLNVAL != 0 {
			return errors.New("terminal input file descriptor is invalid")
		}

		if fds[0].Revents != 0 {
			return ctx.Err()
		}
	}
}
