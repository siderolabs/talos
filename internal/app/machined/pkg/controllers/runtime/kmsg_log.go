// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-kmsg"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	networkutils "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/utils"
	machinedruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	drainTimeout    = 100 * time.Millisecond
	logSendTimeout  = 5 * time.Second
	logRetryTimeout = 1 * time.Second
	logCloseTimeout = 5 * time.Second
)

// KmsgLogDeliveryController watches events and forwards them to the events sink server
// if it's configured.
type KmsgLogDeliveryController struct {
	Drainer *machinedruntime.Drainer

	drainSub *machinedruntime.DrainSubscription
}

// Name implements controller.Controller interface.
func (ctrl *KmsgLogDeliveryController) Name() string {
	return "runtime.KmsgLogDeliveryController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KmsgLogDeliveryController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *KmsgLogDeliveryController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *KmsgLogDeliveryController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if err := networkutils.WaitForNetworkReady(ctx, r,
		func(status *network.StatusSpec) bool {
			return status.AddressReady
		},
		[]controller.Input{
			{
				Namespace: runtime.NamespaceName,
				Type:      runtime.KmsgLogConfigType,
				ID:        optional.Some(runtime.KmsgLogConfigID),
				Kind:      controller.InputWeak,
			},
		},
	); err != nil {
		return fmt.Errorf("error waiting for network: %w", err)
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

		cfg, err := safe.ReaderGetByID[*runtime.KmsgLogConfig](ctx, r, runtime.KmsgLogConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting configuration: %w", err)
		}

		if cfg == nil {
			// no config, wait for the next event
			continue
		}

		if err = ctrl.deliverLogs(ctx, r, logger, kmsgCh, cfg.TypedSpec().Destinations); err != nil {
			return fmt.Errorf("error delivering logs: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

type logConfig struct {
	endpoint *url.URL
}

func (c logConfig) Format() string {
	return constants.LoggingFormatJSONLines
}

func (c logConfig) Endpoint() *url.URL {
	return c.endpoint
}

func (c logConfig) ExtraTags() map[string]string {
	return nil
}

//nolint:gocyclo
func (ctrl *KmsgLogDeliveryController) deliverLogs(ctx context.Context, r controller.Runtime, logger *zap.Logger, kmsgCh <-chan kmsg.Packet, destURLs []*url.URL) error {
	if ctrl.drainSub == nil {
		ctrl.drainSub = ctrl.Drainer.Subscribe()
	}

	// initialize all log senders
	destLogConfigs := xslices.Map(destURLs, func(u *url.URL) config.LoggingDestination {
		return logConfig{endpoint: u}
	})
	senders := xslices.Map(destLogConfigs, logging.NewJSONLines)

	defer func() {
		closeCtx, closeCtxCancel := context.WithTimeout(context.Background(), logCloseTimeout)
		defer closeCtxCancel()

		for _, sender := range senders {
			if err := sender.Close(closeCtx); err != nil {
				logger.Error("error closing log sender", zap.Error(err))
			}
		}
	}()

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

		event := machinedruntime.LogEvent{
			Msg:   msg.Message.Message,
			Time:  msg.Message.Timestamp,
			Level: kmsgPriorityToLevel(msg.Message.Priority),
			Fields: map[string]interface{}{
				"facility": msg.Message.Facility.String(),
				"seq":      msg.Message.SequenceNumber,
				"clock":    msg.Message.Clock,
				"priority": msg.Message.Priority.String(),
			},
		}

		if err := ctrl.resend(ctx, r, logger, senders, &event); err != nil {
			return fmt.Errorf("error sending log event: %w", err)
		}
	}
}

//nolint:gocyclo
func (ctrl *KmsgLogDeliveryController) resend(ctx context.Context, r controller.Runtime, logger *zap.Logger, senders []machinedruntime.LogSender, e *machinedruntime.LogEvent) error {
	for {
		sendCtx, sendCancel := context.WithTimeout(ctx, logSendTimeout)
		sendErrors := make(chan error, len(senders))

		for _, sender := range senders {
			go func() {
				sendErrors <- sender.Send(sendCtx, e)
			}()
		}

		var dontRetry bool

		for range senders {
			err := <-sendErrors

			// don't retry if at least one sender succeed to avoid implementing per-sender queue, etc
			if err == nil {
				dontRetry = true

				continue
			}

			logger.Debug("error sending log event", zap.Error(err))

			if errors.Is(err, machinedruntime.ErrDontRetry) || errors.Is(err, context.Canceled) {
				dontRetry = true
			}
		}

		sendCancel()

		if dontRetry {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			// config changed, restart the loop
			return errors.New("config changed")
		case <-time.After(logRetryTimeout):
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
