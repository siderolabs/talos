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
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// refreshDebounce is the quiet window before a refresh bump.
const refreshDebounce = 250 * time.Millisecond

// refreshMaxWait guarantees bumps under sustained event flow.
const refreshMaxWait = 2 * time.Second

// LVMRefreshTriggerController bumps storage.LVMRefreshRequest whenever any
// block-layer event suggests LVM state may have changed.
type LVMRefreshTriggerController struct {
	V1Alpha1Mode machineruntime.Mode
}

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
	// in container mode, no devices, nothing to scan
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	// Timer stays stopped until first event. nextFire tracks scheduled fire time.
	timer := time.NewTimer(refreshDebounce)
	defer timer.Stop()

	if !timer.Stop() {
		<-timer.C
	}

	var (
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

			// Trailing-edge debounce, capped by refreshMaxWait.
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

		if err := safe.WriterModify(
			ctx, r,
			storage.NewLVMRefreshRequest(storage.NamespaceName, storage.RefreshID),
			func(rr *storage.LVMRefreshRequest) error {
				rr.TypedSpec().Request++

				return nil
			},
		); err != nil {
			return fmt.Errorf("bump LVM refresh request: %w", err)
		}

		pending = false
		firstPending = time.Time{}
	}
}
