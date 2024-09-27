// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package health

import (
	"context"
	"time"
)

// Check runs the health check under given context.
//
// Healthcheck is considered successful when func returns no error.
// Func should terminate when context is canceled.
type Check func(ctx context.Context) error

// Run the health check publishing the results to state.
//
// Run aborts when context is canceled.
func Run(ctx context.Context, settings *Settings, state *State, check Check) error {
	state.Init()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(settings.InitialDelay):
	}

	ticker := time.NewTicker(settings.Period)
	defer ticker.Stop()

	var (
		err            error
		healthy        bool
		message        string
		checkCtx       context.Context
		checkCtxCancel context.CancelFunc
	)

	for {
		err = func() error {
			checkCtx, checkCtxCancel = context.WithTimeout(ctx, settings.Timeout) //nolint:fatcontext
			defer checkCtxCancel()

			return check(checkCtx)
		}()

		healthy = err == nil
		message = ""

		if !healthy {
			message = err.Error()
		}

		state.Update(healthy, message)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
