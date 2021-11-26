// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-kmsg"
	"github.com/talos-systems/go-procfs/procfs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

const drainTimeout = 100 * time.Millisecond

// KmsgLogDeliveryController watches events and forwards them to the events sink server
// if it's configured.
type KmsgLogDeliveryController struct {
	Cmdline *procfs.Cmdline
	Drainer *runtime.Drainer

	drainSub *runtime.DrainSubscription
}

// Name implements controller.Controller interface.
func (ctrl *KmsgLogDeliveryController) Name() string {
	return "runtime.KmsgLogDeliveryController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KmsgLogDeliveryController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.StatusType,
			ID:        pointer.ToString(network.StatusID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KmsgLogDeliveryController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *KmsgLogDeliveryController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) (err error) {
	if ctrl.Cmdline == nil || ctrl.Cmdline.Get(constants.KernelParamLoggingKernel).First() == nil {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var netStatus resource.Resource

		netStatus, err = r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.StatusType, network.StatusID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				// no network state yet
				continue
			}

			return fmt.Errorf("error reading network status: %w", err)
		}

		if !netStatus.(*network.Status).TypedSpec().AddressReady {
			// wait for address
			continue
		}

		break
	}

	if ctrl.drainSub == nil {
		ctrl.drainSub = ctrl.Drainer.Subscribe()
	}

	destURL, err := url.Parse(*ctrl.Cmdline.Get(constants.KernelParamLoggingKernel).First())
	if err != nil {
		return fmt.Errorf("error parsing %q: %w", constants.KernelParamLoggingKernel, err)
	}

	sender := logging.NewJSONLines(destURL)
	defer sender.Close(ctx) //nolint:errcheck

	reader, err := kmsg.NewReader(kmsg.Follow())
	if err != nil {
		return fmt.Errorf("error reading kernel messages: %w", err)
	}

	defer reader.Close() //nolint:errcheck

	kmsgCh := reader.Scan(ctx)

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
		case msg = <-kmsgCh:
			if drainTimer != nil {
				// if draining, reset the timer as there's a new message
				if !drainTimer.Stop() {
					<-drainTimer.C
				}

				drainTimer.Reset(drainTimeout)
			}
		case <-ctrl.drainSub.EventCh():
			// drain started, assume that ksmg is drained if there're no new messages in drainTimeout
			drainTimer = time.NewTimer(drainTimeout)
			drainTimerCh = drainTimer.C
		case <-drainTimerCh:
			ctrl.drainSub.Cancel()

			return nil
		}

		if msg.Err != nil {
			return fmt.Errorf("error receiving kernel logs: %w", msg.Err)
		}

		if err = sender.Send(ctx, &runtime.LogEvent{
			Msg:   msg.Message.Message,
			Time:  msg.Message.Timestamp,
			Level: kmsgPriorityToLevel(msg.Message.Priority),
			Fields: map[string]interface{}{
				"facility": msg.Message.Facility.String(),
				"seq":      msg.Message.SequenceNumber,
				"clock":    msg.Message.Clock,
				"priority": msg.Message.Priority.String(),
			},
		}); err != nil {
			return fmt.Errorf("error sending logs: %w", err)
		}
	}
}

func kmsgPriorityToLevel(pri kmsg.Priority) zapcore.Level {
	switch pri {
	case kmsg.Alert, kmsg.Crit, kmsg.Emerg, kmsg.Err:
		return zapcore.ErrorLevel
	case kmsg.Debug:
		return zapcore.DebugLevel
	case kmsg.Info, kmsg.Notice:
		return zapcore.InfoLevel
	case kmsg.Warning:
		return zapcore.WarnLevel
	default:
		return zapcore.ErrorLevel
	}
}
