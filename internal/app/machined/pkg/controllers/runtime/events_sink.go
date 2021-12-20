// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/rs/xid"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/siderolink/api/events"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// EventsSinkController watches events and forwards them to the events sink server
// if it's configured.
type EventsSinkController struct {
	V1Alpha1Events runtime.Watcher
	Cmdline        *procfs.Cmdline
	Drainer        *runtime.Drainer

	drainSub *runtime.DrainSubscription
	drain    bool
	backlog  atomic.Int32
	eventID  xid.ID
}

// Name implements controller.Controller interface.
func (ctrl *EventsSinkController) Name() string {
	return "v1alpha1.EventsSinkController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EventsSinkController) Inputs() []controller.Input {
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
func (ctrl *EventsSinkController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *EventsSinkController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) (err error) {
	if ctrl.Cmdline == nil || ctrl.Cmdline.Get(constants.KernelParamEventsSink).First() == nil {
		return nil
	}

	if ctrl.drainSub == nil {
		ctrl.backlog.Store(-1)
		ctrl.drainSub = ctrl.Drainer.Subscribe()
	}

	defer func() {
		if ctrl.backlog.Load() == 0 {
			ctrl.drainSub.Cancel()
		}
	}()

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

	errCh := make(chan error)

	sink := ctrl.Cmdline.Get(constants.KernelParamEventsSink).First()

	conn, err := grpc.DialContext(ctx, *sink, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}

	defer conn.Close() //nolint:errcheck

	client := events.NewEventSinkServiceClient(conn)

	opts := []runtime.WatchOptionFunc{}
	if ctrl.eventID.IsNil() {
		opts = append(opts, runtime.WithTailEvents(-1))
	} else {
		opts = append(opts, runtime.WithTailID(ctrl.eventID))
	}

	if err = ctrl.V1Alpha1Events.Watch(func(eventCh <-chan runtime.EventInfo) {
		errCh <- ctrl.handleEvents(ctx, eventCh, client)
	}, opts...); err != nil {
		return err
	}

	err = <-errCh

	return err
}

//nolint:gocyclo
func (ctrl *EventsSinkController) handleEvents(ctx context.Context, eventCh <-chan runtime.EventInfo, client events.EventSinkServiceClient) error {
	for {
		var (
			event runtime.EventInfo
			ok    bool
			data  *anypb.Any
			err   error
		)

		select {
		case <-ctx.Done():
			return nil
		case event, ok = <-eventCh:
			if !ok {
				return nil
			}
		case <-ctrl.drainSub.EventCh():
			backlog := ctrl.backlog.Load()

			if backlog == 0 {
				return nil
			}

			ctrl.drain = true

			continue
		}

		ctrl.backlog.Store(int32(event.Backlog))

		data, err = anypb.New(event.Payload)
		if err != nil {
			return err
		}

		req := &events.EventRequest{
			Id:   event.ID.String(),
			Data: data,
		}

		_, err = client.Publish(ctx, req)
		if err != nil {
			return err
		}

		ctrl.eventID = event.ID

		if ctrl.drain && event.Backlog == 0 {
			return nil
		}
	}
}
