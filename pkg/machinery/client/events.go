// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ErrEventNotSupported is returned from the event decoder when we encounter an unknown event.
var ErrEventNotSupported = errors.New("event is not supported")

// EventsOptionFunc defines the options for the Events API.
type EventsOptionFunc func(opts *machineapi.EventsRequest)

// WithTailEvents sets up Events API to return specified number of past events.
//
// If number is negative, all the available past events are returned.
func WithTailEvents(number int32) EventsOptionFunc {
	return func(opts *machineapi.EventsRequest) {
		opts.TailEvents = number
	}
}

// WithTailID sets up Events API to return events with ID > TailID.
func WithTailID(id string) EventsOptionFunc {
	return func(opts *machineapi.EventsRequest) {
		opts.TailId = id
	}
}

// WithTailDuration sets up Watcher to return events with timestamp >= (now - tailDuration).
func WithTailDuration(dur time.Duration) EventsOptionFunc {
	return func(opts *machineapi.EventsRequest) {
		opts.TailSeconds = int32(dur / time.Second)
	}
}

// WithActorID sets up Watcher to return events with the specified actor ID.
func WithActorID(actorID string) EventsOptionFunc {
	return func(opts *machineapi.EventsRequest) {
		opts.WithActorId = actorID
	}
}

// Events implements the proto.OSClient interface.
func (c *Client) Events(ctx context.Context, opts ...EventsOptionFunc) (stream machineapi.MachineService_EventsClient, err error) {
	var req machineapi.EventsRequest

	for _, opt := range opts {
		opt(&req)
	}

	return c.MachineClient.Events(ctx, &req)
}

// Event as received from the API.
type Event struct {
	Node    string
	TypeURL string
	ID      string
	ActorID string
	Payload proto.Message
}

// EventsWatch wraps Events by providing more simple interface.
//
//nolint:gocyclo
func (c *Client) EventsWatch(ctx context.Context, watchFunc func(<-chan Event), opts ...EventsOptionFunc) error {
	stream, err := c.Events(ctx, opts...)
	if err != nil {
		return fmt.Errorf("error fetching events: %s", err)
	}

	if err = stream.CloseSend(); err != nil {
		return err
	}

	defaultNode := RemotePeer(stream.Context()) //nolint:contextcheck

	var wg sync.WaitGroup

	defer wg.Wait()

	ch := make(chan Event)
	defer close(ch)

	wg.Add(1)

	go func() {
		defer wg.Done()

		watchFunc(ch)
	}()

	for {
		event, err := stream.Recv()
		if err != nil {
			if err == io.EOF || StatusCode(err) == codes.Canceled {
				return nil
			}

			return fmt.Errorf("failed to watch events: %w", err)
		}

		ev, err := UnmarshalEvent(event)
		if err != nil {
			continue
		}

		if ev.Node == "" {
			ev.Node = defaultNode
		}

		select {
		case ch <- *ev:
		case <-ctx.Done():
			return nil
		}
	}
}

// EventResult is the result of an event watch, containing either an Event or an error.
type EventResult struct {
	// Event is the event that was received.
	Event Event
	// Err is the error that occurred.
	Error error
}

// EventsWatchV2 watches events of a single node and wraps the Events by providing a simpler interface.
// It blocks until the first (empty) event is received, then spawns a goroutine that sends events to the given channel.
// EventResult objects sent into the channel contain either the errors or the received events.
//
//nolint:gocyclo
func (c *Client) EventsWatchV2(ctx context.Context, ch chan<- EventResult, opts ...EventsOptionFunc) error {
	ctx, cancel := context.WithCancel(ctx)

	stream, err := c.Events(ctx, opts...)
	if err != nil {
		cancel()

		return fmt.Errorf("error fetching events: %w", err)
	}

	if err = stream.CloseSend(); err != nil {
		cancel()

		return err
	}

	defaultNode := RemotePeer(stream.Context())

	// receive first (empty) watch event
	_, err = stream.Recv()
	if err != nil {
		cancel()

		return fmt.Errorf("error while watching events: %w", err)
	}

	go func() {
		defer cancel()

		err = func() error {
			for {
				event, eventErr := stream.Recv()
				if eventErr != nil {
					return eventErr
				}

				if event.GetMetadata().GetError() != "" {
					var mdErr error
					if event.GetMetadata().GetStatus() != nil {
						mdErr = status.FromProto(event.GetMetadata().GetStatus()).Err()
					} else {
						mdErr = errors.New(event.GetMetadata().GetError())
					}

					return fmt.Errorf("%s: %w", event.GetMetadata().GetHostname(), mdErr)
				}

				ev, eventErr := UnmarshalEvent(event)
				if eventErr != nil {
					return eventErr
				}

				if ev == nil {
					continue
				}

				if ev.Node == "" {
					ev.Node = defaultNode
				}

				select {
				case ch <- EventResult{Event: *ev}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}()
		if err != nil {
			select {
			case ch <- EventResult{Error: err}:
			case <-ctx.Done():
			}
		}
	}()

	return nil
}

// UnmarshalEvent decodes the event coming from the gRPC stream from any to the exact type.
func UnmarshalEvent(event *machineapi.Event) (*Event, error) {
	typeURL := event.GetData().GetTypeUrl()

	var msg proto.Message

	for _, eventType := range []proto.Message{
		&machineapi.SequenceEvent{},
		&machineapi.PhaseEvent{},
		&machineapi.TaskEvent{},
		&machineapi.ServiceStateEvent{},
		&machineapi.ConfigLoadErrorEvent{},
		&machineapi.ConfigValidationErrorEvent{},
		&machineapi.AddressEvent{},
		&machineapi.MachineStatusEvent{},
	} {
		if typeURL == "talos/runtime/"+string(eventType.ProtoReflect().Descriptor().FullName()) {
			msg = eventType

			break
		}
	}

	if msg == nil {
		// We haven't implemented the handling of this event yet.
		return nil, ErrEventNotSupported
	}

	if err := proto.Unmarshal(event.GetData().GetValue(), msg); err != nil {
		log.Printf("failed to unmarshal message: %v", err)

		return nil, err
	}

	ev := Event{
		TypeURL: typeURL,
		ID:      event.Id,
		Payload: msg,
		ActorID: event.ActorId,
	}

	if event.Metadata != nil {
		ev.Node = event.Metadata.Hostname
	}

	return &ev, nil
}
