// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"

	"github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

// Events represents the runtime event stream.
type Events struct {
	// stream is used as ring buffer of events
	stream []machine.Event

	// writePos is the index in streams for the next write (publish)
	writePos int

	// gen tracks number of wraparounds in stream
	gen int64

	// cap is capacity of streams
	cap int

	// mutext protects access to writePos, gen and stream
	mu sync.Mutex
	c  *sync.Cond
}

// NewEvents initializes and returns the v1alpha1 runtime event stream.
func NewEvents(cap int) *Events {
	e := &Events{
		stream: make([]machine.Event, cap),
		cap:    cap,
	}

	e.c = sync.NewCond(&e.mu)

	return e
}

// Watch implements the Events interface.
func (e *Events) Watch(f runtime.WatchFunc) {
	ctx, ctxCancel := context.WithCancel(context.Background())

	ch := make(chan machine.Event)

	go func() {
		defer ctxCancel()

		f(ch)
	}()

	e.mu.Lock()
	pos := e.writePos
	gen := e.gen
	e.mu.Unlock()

	go func() {
		defer close(ch)

		for {
			e.mu.Lock()
			for pos == e.writePos {
				e.c.Wait()

				select {
				case <-ctx.Done():
					e.mu.Unlock()
					return
				default:
				}
			}

			if e.gen > gen && pos < e.writePos {
				// buffer overrun, there's no way to signal error in this case,
				// so for now just return
				e.mu.Unlock()
				return
			}

			event := e.stream[pos]
			pos = (pos + 1) % e.cap
			if pos == 0 {
				gen = e.gen
			}
			e.mu.Unlock()

			select {
			case ch <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Publish implements the Events interface.
func (e *Events) Publish(msg proto.Message) {
	value, err := proto.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal message: %v", err)

		return
	}

	event := machine.Event{
		Data: &any.Any{
			// In the future, we can publish `talos/runtime`, and
			// `talos/plugin/<plugin>` (or something along those lines) events.
			TypeUrl: fmt.Sprintf("talos/runtime/%s", proto.MessageName(msg)),
			Value:   value,
		},
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.stream[e.writePos] = event
	e.writePos = (e.writePos + 1) % e.cap
	if e.writePos == 0 {
		e.gen++
	}

	e.c.Broadcast()
}
