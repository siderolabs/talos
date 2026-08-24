// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/siderolabs/gen/optional"
)

// runWithWakeTimer drives a controller's reconcile loop, waking it on a COSI event or on the
// duration reconcile last asked to be woken after — e.g. dependsOn.paths polling, which has no event
// of its own.
//
// A single timer serves every such deadline: it is reset each pass to whatever reconcile reports
// next, so a controller with nothing left to poll for goes fully idle.
func runWithWakeTimer(
	ctx context.Context,
	r controller.Runtime,
	reconcile func(ctx context.Context, r controller.Runtime) (optional.Optional[time.Duration], error),
) error {
	timer := time.NewTimer(0)
	defer timer.Stop()

	if !timer.Stop() {
		<-timer.C
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-timer.C:
		}

		wakeAfter, err := reconcile(ctx, r)
		if err != nil {
			return err
		}

		if !timer.Stop() {
			// Drain a timer that fired while we were reconciling, so the next Reset is honored.
			select {
			case <-timer.C:
			default:
			}
		}

		if duration, ok := wakeAfter.Get(); ok {
			timer.Reset(duration)
		}

		r.ResetRestartBackoff()
	}
}
