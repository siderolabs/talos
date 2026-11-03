// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
)

// WithContextC wraps function call to provide a context cancellable with SIGTERM (Ctrl^C) or SIGINT.
// Returns the resolved command and error from the function call.
//
// A command can take over the signals with ClaimSignals.
func WithContextC(ctx context.Context, f func(context.Context) (*cobra.Command, error)) (*cobra.Command, error) {
	router := &signalRouter{}

	wrappedCtx, wrappedCtxCancel := context.WithCancel(context.WithValue(ctx, signalRouterKey{}, router))
	defer wrappedCtxCancel()

	// listen for ^C and SIGTERM and abort context
	sigCh := make(chan os.Signal, 1)

	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	exited := make(chan struct{})
	defer close(exited)

	go func() {
		for {
			select {
			case sig := <-sigCh:
				if router.forward(sig) {
					continue
				}

				wrappedCtxCancel()

				signal.Stop(sigCh)
				fmt.Fprintln(os.Stderr, "Signal received, aborting, press Ctrl+C once again to abort immediately...")

				return
			case <-wrappedCtx.Done():
				return
			case <-exited:
				return
			}
		}
	}()

	return f(wrappedCtx)
}

// ClaimSignals delivers SIGINT, SIGTERM and the extra signals to c until release is called, instead of
// SIGINT and SIGTERM canceling the context of WithContextC.
//
// It is meant for commands which forward signals to something else, e.g. a remote process.
func ClaimSignals(ctx context.Context, c chan os.Signal, extra ...os.Signal) (release func()) {
	router, ok := ctx.Value(signalRouterKey{}).(*signalRouter)
	if !ok {
		signal.Notify(c, append([]os.Signal{os.Interrupt, syscall.SIGTERM}, extra...)...)

		return func() { signal.Stop(c) }
	}

	router.claim(c)

	if len(extra) > 0 {
		signal.Notify(c, extra...)
	}

	return func() {
		signal.Stop(c)
		router.claim(nil)
	}
}

type signalRouterKey struct{}

// signalRouter hands the signals of WithContextC to the command which claimed them, if any.
type signalRouter struct {
	mu      sync.Mutex
	claimed chan os.Signal
}

func (r *signalRouter) claim(c chan os.Signal) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.claimed = c
}

func (r *signalRouter) forward(sig os.Signal) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.claimed == nil {
		return false
	}

	select {
	case r.claimed <- sig:
	default:
	}

	return true
}
