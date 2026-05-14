// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// refreshDebounce is the quiet window the trigger waits for after the last
// observed block-layer event before emitting a fresh LVMRefreshRequest.
const refreshDebounce = 250 * time.Millisecond

// refreshMaxWait caps the time the trigger will keep deferring a bump
// under a sustained event stream. Without it, a continuous flow of
// block-layer events (e.g. a controller actively reshaping disks) would
// keep resetting the debounce timer indefinitely and the scan controller
// would never see updated state. With it, the trigger is guaranteed to
// emit at least one bump every refreshMaxWait while events keep arriving.
const refreshMaxWait = 2 * time.Second

// LVMRefreshTriggerController bumps storage.LVMRefreshRequest whenever any
// block-layer event suggests LVM state may have changed.
type LVMRefreshTriggerController struct{}

// Name implements controller.Controller interface.
func (ctrl *LVMRefreshTriggerController) Name() string {
	return "storage.LVMRefreshTriggerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LVMRefreshTriggerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.DeviceType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiskType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.DiscoveredVolumeType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.LVMRefreshStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LVMRefreshTriggerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.LVMRefreshRequestType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *LVMRefreshTriggerController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	// Timer is created in stopped state - it is armed only when an event
	// arrives. nextFire holds the scheduled wall-clock fire time so we
	// can decide whether an event should advance the timer or whether the
	// max-wait deadline has already been hit.
	timer := time.NewTimer(refreshDebounce)
	defer timer.Stop()

	if !timer.Stop() {
		<-timer.C
	}

	var (
		requested    int       // counter value of the last bump we wrote
		pending      bool      // events arrived since the last bump landed
		firstPending time.Time // when the current pending burst began
		nextFire     time.Time // when the timer is currently scheduled to fire
	)

	rearm := func(d time.Duration) {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}

		timer.Reset(d)
		nextFire = time.Now().Add(d)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			now := time.Now()

			if !pending {
				pending = true
				firstPending = now
			}

			// Trailing-edge debounce, capped by refreshMaxWait. Each event
			// extends the quiet window by refreshDebounce, but never past
			// the firstPending + refreshMaxWait deadline - that bounds the
			// worst-case delay under a sustained event flow.
			deadline := firstPending.Add(refreshMaxWait)

			target := now.Add(refreshDebounce)
			if target.After(deadline) {
				target = deadline
			}

			if nextFire.IsZero() || target.Before(nextFire) {
				rearm(max(0, target.Sub(now)))
			}

			continue
		case <-timer.C:
			nextFire = time.Time{}
		}

		if !pending {
			continue
		}

		status, err := safe.ReaderGetByID[*storage.LVMRefreshStatus](ctx, r, storage.RefreshID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("read LVM refresh status: %w", err)
		}

		var observed int
		if status != nil {
			observed = status.TypedSpec().Request
		}

		// Echo gate: if a previous bump has not been echoed by the scan
		// controller yet, defer this bump and re-arm the timer. The scan
		// controller will publish LVMRefreshStatus when it finishes; that
		// arrives as an EventCh wake-up and the quiet window starts again.
		if requested != 0 && observed < requested {
			rearm(refreshDebounce)

			continue
		}

		if err := safe.WriterModify(
			ctx, r,
			storage.NewLVMRefreshRequest(storage.NamespaceName, storage.RefreshID),
			func(rr *storage.LVMRefreshRequest) error {
				rr.TypedSpec().Request++
				requested = rr.TypedSpec().Request

				return nil
			},
		); err != nil {
			return fmt.Errorf("bump LVM refresh request: %w", err)
		}

		pending = false
		firstPending = time.Time{}
	}
}
