// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

func unmountLoop(ctx context.Context, printer func(string, ...any), target string, flags int, timeout time.Duration, extraMessage string) (bool, error) {
	errCh := make(chan error, 1)

	go func() {
		errCh <- unix.Unmount(target, flags)
	}()

	start := time.Now()

	progressTicker := time.NewTicker(timeout / 5)
	defer progressTicker.Stop()

unmountLoop:
	for {
		select {
		case <-ctx.Done():
			return true, ctx.Err()
		case err := <-errCh:
			return true, err
		case <-progressTicker.C:
			timeLeft := timeout - time.Since(start)

			if timeLeft <= 0 {
				break unmountLoop
			}

			if printer != nil {
				printer("unmounting %s%s is taking longer than expected, still waiting for %s", target, extraMessage, timeLeft)
			}
		}
	}

	return false, nil
}

// SafeUnmount unmounts the target path, first without force, then with force if the first attempt fails.
//
// It makes sure that unmounting has a finite operation timeout.
func SafeUnmount(ctx context.Context, printer func(string, ...any), target string) error {
	const (
		unmountTimeout      = 90 * time.Second
		unmountForceTimeout = 10 * time.Second
	)

	ok, err := unmountLoop(ctx, printer, target, 0, unmountTimeout, "")

	if ok {
		return err
	}

	if printer != nil {
		printer("unmounting %s with force", target)
	}

	ok, err = unmountLoop(ctx, printer, target, unix.MNT_FORCE, unmountTimeout, " with force flag")

	if ok {
		return err
	}

	return fmt.Errorf("unmounting %s with force flag timed out", target)
}
