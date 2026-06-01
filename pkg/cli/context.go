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

	"github.com/spf13/cobra"
)

// WithContextC wraps function call to provide a context cancellable with SIGTERM (Ctrl^C) or SIGINT.
// Returns the resolved command and error from the function call.
func WithContextC(ctx context.Context, f func(context.Context) (*cobra.Command, error)) (*cobra.Command, error) {
	wrappedCtx, wrappedCtxCancel := context.WithCancel(ctx)
	defer wrappedCtxCancel()

	// listen for ^C and SIGTERM and abort context
	sigCh := make(chan os.Signal, 1)

	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

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
