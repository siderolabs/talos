// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// WithContext wraps function call to provide a context cancellable with ^C.
func WithContext(ctx context.Context, f func(context.Context) error) error {
	wrappedCtx, wrappedCtxCancel := context.WithCancel(ctx)
	defer wrappedCtxCancel()

	// listen for ^C and SIGTERM and abort context
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	exited := make(chan struct{})
	defer close(exited)

	go func() {
		select {
		case <-sigCh:
			wrappedCtxCancel()

			signal.Stop(sigCh)
			fmt.Fprintln(os.Stderr, "Signal received, aborting, press Ctrl+C once again to abort immediately...")
		case <-wrappedCtx.Done():
			return
		case <-exited:
		}
	}()

	return f(wrappedCtx)
}
