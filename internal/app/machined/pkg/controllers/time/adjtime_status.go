// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package time

import (
	"context"
	"fmt"
	stdtime "time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/timex"
	"github.com/siderolabs/talos/pkg/machinery/resources/time"
)

// AdjtimeStatusController manages time.AdjtimeStatus based on Linux kernel info.
type AdjtimeStatusController struct {
	V1Alpha1Mode v1alpha1runtime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *AdjtimeStatusController) Name() string {
	return "time.AdjtimeStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *AdjtimeStatusController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *AdjtimeStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: time.AdjtimeStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *AdjtimeStatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if ctrl.V1Alpha1Mode == v1alpha1runtime.ModeContainer {
		// in container mode, clock is managed by the host
		return nil
	}

	const pollInterval = 30 * stdtime.Second

	pollTicker := stdtime.NewTicker(pollInterval)
	defer pollTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-pollTicker.C:
		}

		var timexBuf unix.Timex

		state, err := timex.Adjtimex(&timexBuf)
		if err != nil {
			return fmt.Errorf("failed to get adjtimex state: %w", err)
		}

		scale := stdtime.Nanosecond

		if timexBuf.Status&unix.STA_NANO == 0 {
			scale = stdtime.Microsecond
		}

		if err := safe.WriterModify(ctx, r, time.NewAdjtimeStatus(), func(status *time.AdjtimeStatus) error {
			status.TypedSpec().Offset = stdtime.Duration(timexBuf.Offset) * scale //nolint:durationcheck
			status.TypedSpec().FrequencyAdjustmentRatio = 1 + float64(timexBuf.Freq)/65536.0/1000000.0
			status.TypedSpec().MaxError = stdtime.Duration(timexBuf.Maxerror) * stdtime.Microsecond //nolint:durationcheck
			status.TypedSpec().EstError = stdtime.Duration(timexBuf.Esterror) * stdtime.Microsecond //nolint:durationcheck
			status.TypedSpec().Status = timex.Status(timexBuf.Status).String()
			status.TypedSpec().State = state.String()
			status.TypedSpec().Constant = int(timexBuf.Constant)
			status.TypedSpec().SyncStatus = timexBuf.Status&unix.STA_UNSYNC == 0

			return nil
		}); err != nil {
			return fmt.Errorf("failed to update adjtime status: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
