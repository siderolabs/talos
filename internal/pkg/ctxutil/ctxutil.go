// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ctxutil provides utilities for working with contexts.
package ctxutil

import "context"

// MonitorFn starts a function in a new goroutine and cancels the context with error as cause when the function returns.
// It returns the new context.
func MonitorFn(ctx context.Context, fn func() error) context.Context {
	ctx, cancel := context.WithCancelCause(ctx)

	go func() { cancel(fn()) }()

	return ctx
}

// Cause returns the cause of the context error, or nil if there is no error or the error is a usual context error.
func Cause(ctx context.Context) error {
	if c := context.Cause(ctx); c != ctx.Err() {
		return c
	}

	return nil
}
