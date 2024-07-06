// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package events

import (
	"time"

	"github.com/siderolabs/gen/xslices"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/health"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
)

// MaxEventsToKeep is maximum number of events to keep per service before dropping old entries.
const MaxEventsToKeep = 64

// ServiceState is enum of service run states.
type ServiceState int

// ServiceState constants.
const (
	StateInitialized ServiceState = iota
	StatePreparing
	StateWaiting
	StateRunning
	StateStopping
	StateFinished
	StateFailed
	StateSkipped
	StateStarting
)

func (state ServiceState) String() string {
	switch state {
	case StateInitialized:
		return "Initialized"
	case StateStarting:
		return "Starting"
	case StatePreparing:
		return "Preparing"
	case StateWaiting:
		return "Waiting"
	case StateRunning:
		return "Running"
	case StateStopping:
		return "Stopping"
	case StateFinished:
		return "Finished"
	case StateFailed:
		return "Failed"
	case StateSkipped:
		return "Skipped"
	default:
		return "Unknown"
	}
}

// ServiceEvent describes state change of the running service.
type ServiceEvent struct {
	Message   string
	State     ServiceState
	Health    health.Status
	Timestamp time.Time
}

// AsProto returns protobuf representation of respective machined event.
func (event *ServiceEvent) AsProto(service string) *machineapi.ServiceStateEvent {
	return &machineapi.ServiceStateEvent{
		Service: service,
		Action:  machineapi.ServiceStateEvent_Action(event.State),
		Message: event.Message,
		Health:  event.Health.AsProto(),
	}
}

// ServiceEvents is a fixed length history of events.
type ServiceEvents struct {
	events    []ServiceEvent
	pos       int
	discarded uint
}

// Push appends new event to the history popping out oldest event on overflow.
func (events *ServiceEvents) Push(event ServiceEvent) {
	if events.events == nil {
		events.events = make([]ServiceEvent, MaxEventsToKeep)
	}

	if events.events[events.pos].Message != "" {
		// overwriting some entry
		events.discarded++
	}

	events.events[events.pos] = event
	events.pos = (events.pos + 1) % len(events.events)
}

// Get return a copy of event history, with most recent event being the last one.
func (events *ServiceEvents) Get(count int) (result []ServiceEvent) {
	if events.events == nil {
		return
	}

	if count > MaxEventsToKeep {
		count = MaxEventsToKeep
	}

	n := len(events.events)

	for i := (events.pos - count + n) % n; count > 0; i = (i + 1) % n {
		if events.events[i].Message != "" {
			result = append(result, events.events[i])
		}

		count--
	}

	return
}

// AsProto returns protobuf-ready serialized snapshot.
func (events *ServiceEvents) AsProto(count int) *machineapi.ServiceEvents {
	eventList := events.Get(count)

	fn := func(event ServiceEvent) *machineapi.ServiceEvent {
		tspb := timestamppb.New(event.Timestamp)

		return &machineapi.ServiceEvent{
			Msg:   event.Message,
			State: event.State.String(),
			Ts:    tspb,
		}
	}

	return &machineapi.ServiceEvents{
		Events: xslices.Map(eventList, fn),
	}
}

// Recorder adds new event to the history of events, formatting message with args using Sprintf.
type Recorder func(newstate ServiceState, message string, args ...any)

// NullRecorder discards events.
func NullRecorder(newstate ServiceState, message string, args ...any) {
}
