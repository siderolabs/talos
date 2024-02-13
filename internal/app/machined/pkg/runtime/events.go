// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/xid"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ActorIDCtxKey is the context key used for event actor id.
type ActorIDCtxKey struct{}

// Event is what is sent on the wire.
type Event struct {
	TypeURL string
	ID      xid.ID
	Payload proto.Message
	ActorID string
}

// EventInfo unifies event and queue information for the WatchFunc.
type EventInfo struct {
	Event
	Backlog int
}

// WatchFunc defines the watcher callback function.
type WatchFunc func(<-chan EventInfo)

// WatchOptions defines options for the watch call.
//
// Only one of TailEvents, TailID or TailDuration should be non-zero.
type WatchOptions struct {
	// Return that many past events.
	//
	// If TailEvents is negative, return all the events available.
	TailEvents int
	// Start at ID > specified.
	TailID xid.ID
	// Start at timestamp Now() - TailDuration.
	TailDuration time.Duration
	// ActorID to ID of the actor to filter events by.
	ActorID string
}

// WatchOptionFunc defines the options for the watcher.
type WatchOptionFunc func(opts *WatchOptions) error

// WithTailEvents sets up Watcher to return specified number of past events.
//
// If number is negative, all the available past events are returned.
func WithTailEvents(number int) WatchOptionFunc {
	return func(opts *WatchOptions) error {
		if !opts.TailID.IsNil() || opts.TailDuration != 0 {
			return errors.New("WithTailEvents can't be specified at the same time with WithTailID or WithTailDuration")
		}

		opts.TailEvents = number

		return nil
	}
}

// WithTailID sets up Watcher to return events with ID > TailID.
func WithTailID(id xid.ID) WatchOptionFunc {
	return func(opts *WatchOptions) error {
		if opts.TailEvents != 0 || opts.TailDuration != 0 {
			return errors.New("WithTailID can't be specified at the same time with WithTailEvents or WithTailDuration")
		}

		opts.TailID = id

		return nil
	}
}

// WithTailDuration sets up Watcher to return events with timestamp >= (now - tailDuration).
func WithTailDuration(dur time.Duration) WatchOptionFunc {
	return func(opts *WatchOptions) error {
		if opts.TailEvents != 0 || !opts.TailID.IsNil() {
			return errors.New("WithTailDuration can't be specified at the same time with WithTailEvents or WithTailID")
		}

		opts.TailDuration = dur

		return nil
	}
}

// WithActorID sets up Watcher to return events filtered by given actor id.
func WithActorID(actorID string) WatchOptionFunc {
	return func(opts *WatchOptions) error {
		opts.ActorID = actorID

		return nil
	}
}

// Watcher defines a runtime event watcher.
type Watcher interface {
	Watch(WatchFunc, ...WatchOptionFunc) error
}

// Publisher defines a runtime event publisher.
type Publisher interface {
	Publish(context.Context, proto.Message)
}

// EventStream defines the runtime event stream.
type EventStream interface {
	Watcher
	Publisher
}

// NewEvent creates a new event with the provided payload and actor ID.
func NewEvent(payload proto.Message, actorID string) Event {
	typeURL := ""
	if payload != nil {
		typeURL = fmt.Sprintf("talos/runtime/%s", payload.ProtoReflect().Descriptor().FullName())
	}

	return Event{
		// In the future, we can publish `talos/runtime`, and
		// `talos/plugin/<plugin>` (or something along those lines) events.
		// TypeURL: fmt.Sprintf("talos/runtime/%s", protoreflect.MessageDescriptor.FullName(msg)),
		TypeURL: typeURL,
		Payload: payload,
		ID:      xid.New(),
		ActorID: actorID,
	}
}

// ToMachineEvent serializes Event as proto message machine.Event.
func (event *Event) ToMachineEvent() (*machine.Event, error) {
	value, err := proto.Marshal(event.Payload)
	if err != nil {
		return nil, err
	}

	return &machine.Event{
		Data: &anypb.Any{
			TypeUrl: event.TypeURL,
			Value:   value,
		},
		Id:      event.ID.String(),
		ActorId: event.ActorID,
	}, nil
}
