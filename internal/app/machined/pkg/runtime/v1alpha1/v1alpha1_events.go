// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"fmt"
	"sync"

	"github.com/golang/protobuf/proto"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

// Events represents the runtime event stream.
//
// Events internally is implemented as circular buffer of `runtime.Event`.
// `e.stream` slice is allocated to the initial capacity and slice size doesn't change
// throughout the lifetime of Events.
//
// To explain the internals, let's call `Publish()` method 'Publisher' (there might be
// multiple callers for it), and each `Watch()` handler as 'Consumer'.
//
// For Publisher, `Events` keeps `e.writePos` and `e.gen`, `e.writePos` is index into
// `e.stream` to write the next event to. After the write `e.writePos` is incremented.
// As `e.writePos` goes above capacity-1, it wraps around  to zero and `e.gen` is incremented.
//
// So at any time `0 <= e.writePos < e.cap`, but `e.gen` indicates how many times `e.stream` slice
// was overwritten.
//
// Each Consumer captures initial position it starts consumption from as `pos` and `gen` which are
// local to each Consumers, as Consumers are free to work on their own pace. Following diagram shows
// Publisher and three Consumers:
//
//                                                 Consumer 3                         Consumer 2
//                                                 pos = 9                            pos = 16
//                                                 gen = 1                            gen = 1
//  e.stream []Event                               |                                  |
//                                                 |                                  |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  | 0  | 1  | 2  | 3  | 4  | 5  | 6  | 7  | 8  | 9  | 10 | 11 | 12 | 13 | 14 | 15 | 16 |17  |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//                                       |                                  |
//                                       |                                  |
//                                       Consumer 1                         Publisher
//                                       pos = 7                            e.writePos = 14
//                                       gen = 2                            e.gen = 2
//
// Capacity of Events in this diagram is 18, Publisher published already 14 + 2 * 18 = 40 events, so it
// already overwrote `e.stream` twice fully (e.gen = 2).
//
// Consumer1 is trying to keep up with the publisher, it's on the same `gen`, and it has 14-7 = 7 events
// to catch up.
//
// Consumer2 is on `gen` 1, so it is reading events which were published before the Publisher did last
// wraparound for `e.writePos` at `e.gen == 1`. Consumer 2 has a lot of events to catch up, but as it stays
// on track, it can still do that.
//
// Consumer3 is doing bad - it's on `gen` 1, but Publisher already overwrote this element while going on `gen` 2,
// so Consumer3 is handling incorrect data, it should error out.
//
// Synchronization: at the moment single mutex protects `e.stream`, `e.writePos` and `e.gen`, consumers keep their
// position as local variable, so it doesn't require synchronization. If Consumer catches up with Publisher,
// it sleeps on condition variable to be woken up by Publisher on next publish.
type Events struct {
	// stream is used as ring buffer of events
	stream []runtime.Event

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
		stream: make([]runtime.Event, cap),
		cap:    cap,
	}

	e.c = sync.NewCond(&e.mu)

	return e
}

// Watch implements the Events interface.
//
//nolint: gocyclo
func (e *Events) Watch(f runtime.WatchFunc) {
	// context is used to abort the loop when WatchFunc exits
	ctx, ctxCancel := context.WithCancel(context.Background())

	ch := make(chan runtime.Event)

	go func() {
		defer ctxCancel()

		f(ch)
	}()

	// capture initial consumer position/gen, consumer starts consuming from the next
	// event to be published
	e.mu.Lock()
	pos := e.writePos
	gen := e.gen
	e.mu.Unlock()

	go func() {
		defer close(ch)

		for {
			e.mu.Lock()
			// while there's no data to consume (pos == e.writePos), wait for Condition variable signal,
			// then recheck the condition to be true.
			for pos == e.writePos {
				e.c.Wait()

				select {
				case <-ctx.Done():
					e.mu.Unlock()
					return
				default:
				}
			}

			if e.gen > gen+1 || (e.gen > gen && pos < e.writePos) {
				// buffer overrun, there's no way to signal error in this case,
				// so for now just return
				//
				// why buffer overrun?
				//  if gen is 2 generations behind of e.gen, buffer was overwritten anyways
				//  if gen is 1 generation behind of e.gen, buffer was overwritten if consumer
				//    is behind the publisher.
				e.mu.Unlock()
				return
			}

			event := e.stream[pos]
			pos = (pos + 1) % e.cap

			if pos == 0 {
				// consumer wraps around e.cap-1, so increment gen
				gen++
			}

			e.mu.Unlock()

			// send event to WatchFunc, wait for it to process the event
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
	event := runtime.Event{
		// In the future, we can publish `talos/runtime`, and
		// `talos/plugin/<plugin>` (or something along those lines) events.
		TypeURL: fmt.Sprintf("talos/runtime/%s", proto.MessageName(msg)),
		Payload: msg,
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.stream[e.writePos] = event
	e.writePos = (e.writePos + 1) % e.cap

	if e.writePos == 0 {
		// wraparound around e.cap-1, increment generation
		e.gen++
	}

	e.c.Broadcast()
}
