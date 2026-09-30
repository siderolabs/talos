// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build darwin

package terminal

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

const inputWaitInterval = 100 * time.Millisecond

func waitReadable(ctx context.Context, fd uintptr) error {
	var descriptorSet unix.FdSet

	maxFD := uintptr(len(descriptorSet.Bits) * 32)
	if fd >= maxFD {
		return fmt.Errorf("terminal input file descriptor %d exceeds select limit %d", fd, maxFD-1)
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		descriptorSet.Zero()
		descriptorSet.Set(int(fd))
		timeout := unix.NsecToTimeval(inputWaitInterval.Nanoseconds())

		n, err := unix.Select(int(fd)+1, &descriptorSet, nil, nil, &timeout)
		if errors.Is(err, unix.EINTR) {
			continue
		}

		if err != nil {
			return fmt.Errorf("select terminal input: %w", err)
		}

		if n > 0 && descriptorSet.IsSet(int(fd)) {
			return ctx.Err()
		}
	}
}
