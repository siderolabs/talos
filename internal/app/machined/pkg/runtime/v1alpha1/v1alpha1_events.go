// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rs/xid"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/proto"
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
// For Publisher, `Events` keeps `e.writePos`, `e.writePos` is write offset into `e.stream`.
// Offset `e.writePos` is always incremeneted, real write index is `e.writePos % e.cap`
//
// Each Consumer captures initial position it starts consumption from as `pos` which is
// local to each Consumer, as Consumers are free to work on their own pace. Following diagram shows
// Publisher and three Consumers:
//
//                                                 Consumer 3                         Consumer 2
//                                                 pos = 27                           pos = 34
//  e.stream []Event                               |                                  |
//                                                 |                                  |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//  | 0  | 1  | 2  | 3  | 4  | 5  | 6  | 7  | 8  | 9  | 10 | 11 | 12 | 13 | 14 | 15 | 16 |17  |
//  +----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+----+
//                                       |                                  |
//                                       |                                  |
//                                       Consumer 1                         Publisher
//                                       pos = 43                           e.writePos = 50
//
// Capacity of Events in this diagram is 18, Publisher published already 50 events, so it
// already overwrote `e.stream` twice fully.
//
// Consumer1 is trying to keep up with the publisher, it has 14-7 = 7 events to catch up.
//
// Consumer2 is reading events published by Publisher before last wraparound, it has
// 50-34 = 16 events to catch up. Consumer 2 has a lot of events to catch up, but as it stays
// on track, it can still do that.
//
// Consumer3 is doing bad: 50-27 = 23 > 18 (capacity), so its read position has already been
// overwritten, it can't read consistent data,  soit should error out.
//
// Synchronization: at the moment single mutex protects `e.stream`  and `e.writePos`, consumers keep their
// position as local variable, so it doesn't require synchronization. If Consumer catches up with Publisher,
// it sleeps on condition variable to be woken up by Publisher on next publish.
type Events struct {
	// stream is used as ring buffer of events
	stream []runtime.Event

	// writePos is the index in streams for the next write (publish)
	//
	// writePos gets always incremented, real position in slice is (writePos % cap)
	writePos int64

	// cap is a capacity of the stream
	cap int
	// gap is a safety gap between consumers and publishers
	gap int

	// mutext protects access to writePos and stream
	mu sync.Mutex
	c  *sync.Cond
}

// NewEvents initializes and returns the v1alpha1 runtime event stream.
//
// Argument cap is a maximum event stream capacity (available event history).
// Argument gap is a safety gap to separate consumer from the publisher.
// Maximum available event history is (cap-gap).
func NewEvents(capacity, gap int) *Events {
	e := &Events{
		stream: make([]runtime.Event, capacity),
		cap:    capacity,
		gap:    gap,
	}

	if gap >= capacity {
		// we should never reach this, but if we do, panic so that we know.
		panic("NewEvents: gap >= capacity")
	}

	e.c = sync.NewCond(&e.mu)

	return e
}

// Watch implements the Events interface.
//
//nolint:gocyclo
func (e *Events) Watch(f runtime.WatchFunc, opt ...runtime.WatchOptionFunc) error {
	var opts runtime.WatchOptions

	for _, o := range opt {
		if err := o(&opts); err != nil {
			return err
		}
	}

	// context is used to abort the loop when WatchFunc exits
	ctx, ctxCancel := context.WithCancel(context.Background())

	ch := make(chan runtime.EventInfo)

	go func() {
		defer ctxCancel()

		f(ch)
	}()

	e.mu.Lock()

	// capture initial consumer position: by default, consumer starts consuming from the next
	// event to be published
	pos := e.writePos
	minPos := e.writePos - int64(e.cap-e.gap)

	if minPos < 0 {
		minPos = 0
	}

	// calculate initial position based on options
	switch {
	case opts.TailEvents != 0:
		if opts.TailEvents < 0 {
			pos = minPos
		} else {
			pos -= int64(opts.TailEvents)

			if pos < minPos {
				pos = minPos
			}
		}
	case !opts.TailID.IsNil():
		pos = minPos + int64(sort.Search(int(pos-minPos), func(i int) bool {
			event := e.stream[(minPos+int64(i))%int64(e.cap)]

			return event.ID.Compare(opts.TailID) > 0
		}))
	case opts.TailDuration != 0:
		timestamp := time.Now().Add(-opts.TailDuration)

		pos = minPos + int64(sort.Search(int(pos-minPos), func(i int) bool {
			event := e.stream[(minPos+int64(i))%int64(e.cap)]

			return event.ID.Time().After(timestamp)
		}))
	}

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

			if e.writePos-pos >= int64(e.cap) {
				// buffer overrun, there's no way to signal error in this case,
				// so for now just return
				e.mu.Unlock()

				return
			}

			event := e.stream[pos%int64(e.cap)]
			pos++
			backlog := int(e.writePos - pos)

			e.mu.Unlock()

			// send event to WatchFunc, wait for it to process the event
			select {
			case ch <- runtime.EventInfo{
				Event:   event,
				Backlog: backlog,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// Publish implements the Events interface.
func (e *Events) Publish(msg proto.Message) {
	event := runtime.Event{
		// In the future, we can publish `talos/runtime`, and
		// `talos/plugin/<plugin>` (or something along those lines) events.
		// TypeURL: fmt.Sprintf("talos/runtime/%s", protoreflect.MessageDescriptor.FullName(msg)),

		TypeURL: fmt.Sprintf("talos/runtime/%s", msg.ProtoReflect().Descriptor().FullName()),
		Payload: msg,
		ID:      xid.New(),
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.stream[e.writePos%int64(e.cap)] = event
	e.writePos++

	e.c.Broadcast()
}
