// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/protobuf/proto"

	"github.com/talos-systems/talos/api/machine"
)

// Event is what is sent on the wire.
type Event struct {
	TypeURL string
	Payload proto.Message
}

// WatchFunc defines the watcher callback function.
type WatchFunc func(<-chan Event)

// Watcher defines a runtime event watcher.
type Watcher interface {
	Watch(WatchFunc)
}

// Publisher defines a runtime event publisher.
type Publisher interface {
	Publish(proto.Message)
}

// EventStream defines the runtime event stream.
type EventStream interface {
	Watcher
	Publisher
}

// ToMachineEvent serializes Event as proto message machine.Event.
func (event *Event) ToMachineEvent() (*machine.Event, error) {
	value, err := proto.Marshal(event.Payload)
	if err != nil {
		return nil, err
	}

	return &machine.Event{
		Data: &any.Any{
			TypeUrl: event.TypeURL,
			Value:   value,
		},
	}, nil
}
