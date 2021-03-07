// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
)

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

	defaultNode := RemotePeer(stream.Context())

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
			if err == io.EOF || status.Code(err) == codes.Canceled {
				return nil
			}

			return fmt.Errorf("failed to watch events: %w", err)
		}

		typeURL := event.GetData().GetTypeUrl()

		var msg proto.Message

		for _, eventType := range []proto.Message{
			&machineapi.SequenceEvent{},
			&machineapi.PhaseEvent{},
			&machineapi.TaskEvent{},
			&machineapi.ServiceStateEvent{},
		} {
			if typeURL == "talos/runtime/"+string(eventType.ProtoReflect().Descriptor().FullName()) {
				msg = eventType

				break
			}
		}

		if msg == nil {
			// We haven't implemented the handling of this event yet.
			continue
		}

		if err = proto.Unmarshal(event.GetData().GetValue(), msg); err != nil {
			log.Printf("failed to unmarshal message: %v", err) // TODO: this should be fixed to return errors

			continue
		}

		ev := Event{
			Node:    defaultNode,
			TypeURL: typeURL,
			ID:      event.Id,
			Payload: msg,
		}

		if event.Metadata != nil {
			ev.Node = event.Metadata.Hostname
		}

		select {
		case ch <- ev:
		case <-ctx.Done():
			return nil
		}
	}
}
