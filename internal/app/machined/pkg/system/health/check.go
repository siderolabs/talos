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

	var startupSucceeded bool

	if settings.StartupWindow > 0 {
		var err error

		startupSucceeded, err = runStartup(ctx, settings, state, check)
		if err != nil {
			return err
		}

		// the startup window might have expired at the same time as the context was canceled
		if err = ctx.Err(); err != nil {
			return err
		}
	}

	ticker := time.NewTicker(settings.Period)
	defer ticker.Stop()

	// if the startup phase reported the service as healthy, wait for the next tick before the next check,
	// otherwise run (and report) the check right away
	skipCheck := startupSucceeded

	for {
		if !skipCheck {
			runAndReport(ctx, settings.Timeout, state, check)
		}

		skipCheck = false

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// runStartup runs the startup phase of the health check.
//
// Failures are not reported, the first successful check is reported and ends the startup phase.
// It returns true if the startup phase ended with a successful check, and false if the startup window expired.
func runStartup(ctx context.Context, settings *Settings, state *State, check Check) (bool, error) {
	period := settings.StartupPeriod
	if period <= 0 {
		period = settings.Period
	}

	timeout := settings.StartupTimeout
	if timeout <= 0 {
		timeout = settings.Timeout
	}

	window := time.NewTimer(settings.StartupWindow)
	defer window.Stop()

	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		if runCheck(ctx, timeout, check) == nil {
			state.Update(true, "")

			return true, nil
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-window.C:
			return false, nil
		case <-ticker.C:
		}
	}
}

// runAndReport runs the health check and reports the result (both success and failure) to the state.
func runAndReport(ctx context.Context, timeout time.Duration, state *State, check Check) {
	err := runCheck(ctx, timeout, check)

	healthy := err == nil
	message := ""

	if !healthy {
		message = err.Error()
	}

	state.Update(healthy, message)
}

func runCheck(ctx context.Context, timeout time.Duration, check Check) error {
	checkCtx, checkCtxCancel := context.WithTimeout(ctx, timeout)
	defer checkCtxCancel()

	return check(checkCtx)
}
