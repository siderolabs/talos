// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/rs/xid"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/siderolink/api/events"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/anypb"

	networkutils "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/utils"
	machinedruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/grpc/dialer"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// EventsSinkController watches events and forwards them to the events sink server
// if it's configured.
type EventsSinkController struct {
	V1Alpha1Events machinedruntime.Watcher
	Drainer        *machinedruntime.Drainer

	drainSub *machinedruntime.DrainSubscription
	eventID  xid.ID
}

// Name implements controller.Controller interface.
func (ctrl *EventsSinkController) Name() string {
	return "v1alpha1.EventsSinkController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EventsSinkController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *EventsSinkController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *EventsSinkController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if ctrl.drainSub == nil {
		ctrl.drainSub = ctrl.Drainer.Subscribe()
	}

	defer func() {
		if ctrl.drainSub != nil {
			ctrl.drainSub.Cancel()
		}
	}()

	if err := networkutils.WaitForNetworkReady(ctx, r,
		func(status *network.StatusSpec) bool {
			return status.AddressReady
		},
		[]controller.Input{
			{
				Namespace: runtime.NamespaceName,
				Type:      runtime.EventSinkConfigType,
				ID:        optional.Some(runtime.EventSinkConfigID),
				Kind:      controller.InputWeak,
			},
		},
	); err != nil {
		return fmt.Errorf("error waiting for network: %w", err)
	}

	var (
		conn                    *grpc.ClientConn
		client                  events.EventSinkServiceClient
		watchCh, consumeWatchCh chan machinedruntime.EventInfo
		backlog                 int
		draining                bool
	)

	defer func() {
		if conn != nil {
			conn.Close() //nolint:errcheck
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ctrl.drainSub.EventCh():
			// drain started, return immediately if there's no backlog
			draining = true

			if backlog == 0 {
				return nil
			}
		case event := <-consumeWatchCh:
			// if consumeWatchCh is not nil, client connection was established
			backlog = event.Backlog

			data, err := anypb.New(event.Payload)
			if err != nil {
				return err
			}

			req := &events.EventRequest{
				Id:      event.ID.String(),
				Data:    data,
				ActorId: event.ActorID,
			}

			_, err = client.Publish(ctx, req)
			if err != nil {
				return fmt.Errorf("error publishing event: %w", err)
			}

			// adjust last consumed event
			ctrl.eventID = event.ID

			// if draining and backlog is 0, return immediately
			if draining && backlog == 0 {
				return nil
			}
		case <-r.EventCh():
			// configuration changed, re-establish connection
			cfg, err := safe.ReaderGetByID[*runtime.EventSinkConfig](ctx, r, runtime.EventSinkConfigID)
			if err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting event sink config: %w", err)
			}

			if conn != nil {
				logger.Debug("closing connection to event sink")

				conn.Close() //nolint:errcheck
				conn = nil
				client = nil
				consumeWatchCh = nil // stop consuming events
				backlog = 0
			}

			if cfg == nil {
				// no config, no event streaming
				continue
			}

			// establish connection
			logger.Debug("establishing connection to event sink", zap.String("endpoint", cfg.TypedSpec().Endpoint))

			conn, err = grpc.NewClient(
				cfg.TypedSpec().Endpoint,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithSharedWriteBuffer(true),
				grpc.WithContextDialer(dialer.DynamicProxyDialer),
			)
			if err != nil {
				return fmt.Errorf("error establishing connection to event sink: %w", err)
			}

			client = events.NewEventSinkServiceClient(conn)

			// start watching events if we haven't already done so
			//
			// watch is only established with the first live connection to make sure we don't miss any events
			if watchCh == nil {
				watchCh = make(chan machinedruntime.EventInfo)

				var opts []machinedruntime.WatchOptionFunc

				if ctrl.eventID.IsNil() {
					opts = append(opts, machinedruntime.WithTailEvents(-1))
				} else {
					opts = append(opts, machinedruntime.WithTailID(ctrl.eventID))
				}

				// Watch returns immediately, setting up a goroutine which will copy events to `watchCh`
				if err = ctrl.V1Alpha1Events.Watch(func(eventCh <-chan machinedruntime.EventInfo) {
					for {
						select {
						case <-ctx.Done():
							return
						case event := <-eventCh:
							if !channel.SendWithContext(ctx, watchCh, event) {
								return
							}
						}
					}
				}, opts...); err != nil {
					return err
				}
			}

			consumeWatchCh = watchCh
		}
	}
}
