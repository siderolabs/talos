// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/siderolabs/go-kmsg"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// KmsgLogStorageController watches events and forwards them to the system logger.
type KmsgLogStorageController struct {
	Drainer         *runtime.Drainer
	V1Alpha1Logging runtime.LoggingManager

	drainSub  *runtime.DrainSubscription
	logWriter io.WriteCloser
}

// Name implements controller.Controller interface.
func (ctrl *KmsgLogStorageController) Name() string {
	return "runtime.KmsgLogStorageController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KmsgLogStorageController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *KmsgLogStorageController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *KmsgLogStorageController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var err error

	ctrl.logWriter, err = ctrl.V1Alpha1Logging.ServiceLog("kernel").Writer()
	if err != nil {
		return fmt.Errorf("error opening logger: %w", err)
	}

	// initilalize kmsg reader early, so that we don't lose position on config changes
	reader, err := kmsg.NewReader(kmsg.Follow())
	if err != nil {
		return fmt.Errorf("error reading kernel messages: %w", err)
	}

	defer reader.Close() //nolint:errcheck

	kmsgCh := reader.Scan(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err = ctrl.deliverLogs(ctx, r, kmsgCh); err != nil {
			return fmt.Errorf("error delivering logs: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo
func (ctrl *KmsgLogStorageController) deliverLogs(ctx context.Context, r controller.Runtime, kmsgCh <-chan kmsg.Packet) error {
	if ctrl.drainSub == nil {
		ctrl.drainSub = ctrl.Drainer.Subscribe()
	}

	var (
		drainTimer   *time.Timer
		drainTimerCh <-chan time.Time
	)

	for {
		var msg kmsg.Packet

		select {
		case <-ctx.Done():
			ctrl.drainSub.Cancel()

			return nil
		case <-r.EventCh():
			// config changed, restart the loop
			return nil
		case <-ctrl.drainSub.EventCh():
			// drain started, assume that ksmg is drained if there're no new messages in drainTimeout
			drainTimer = time.NewTimer(drainTimeout)
			drainTimerCh = drainTimer.C

			continue
		case <-drainTimerCh:
			ctrl.drainSub.Cancel()

			return nil
		case msg = <-kmsgCh:
			if drainTimer != nil {
				// if draining, reset the timer as there's a new message
				if !drainTimer.Stop() {
					<-drainTimer.C
				}

				drainTimer.Reset(drainTimeout)
			}
		}

		if msg.Err != nil {
			return fmt.Errorf("error receiving kernel logs: %w", msg.Err)
		}

		ctrl.logWriter.Write([]byte(
			msg.Message.Timestamp.String() + ": " + msg.Message.Facility.String() + ": " + msg.Message.Message,
		))
	}
}
