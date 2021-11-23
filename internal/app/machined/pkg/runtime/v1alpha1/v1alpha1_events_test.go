// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:scopelint,testpackage
package v1alpha1

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/api/machine"
)

func TestEvents_Publish(t *testing.T) {
	tests := []struct {
		name     string
		cap      int
		watchers int
		messages int
	}{
		{
			name:     "nowatchers",
			cap:      100,
			watchers: 0,
			messages: 100,
		},
		{
			name:     "onemessage",
			cap:      100,
			watchers: 10,
			messages: 1,
		},
		{
			name:     "manymessages_singlewatcher",
			cap:      100,
			watchers: 1,
			messages: 50,
		},
		{
			name:     "manymessages_manywatchers",
			cap:      100,
			watchers: 20,
			messages: 50,
		},
		{
			name:     "manymessages_overcap",
			cap:      10,
			watchers: 5,
			messages: 200,
		},
		{
			name:     "megamessages_overcap",
			cap:      1000,
			watchers: 1,
			messages: 2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEvents(tt.cap, tt.cap/10)

			var wg sync.WaitGroup
			wg.Add(tt.watchers)

			got := uint32(0)

			for i := 0; i < tt.watchers; i++ {
				if err := e.Watch(func(events <-chan runtime.EventInfo) {
					defer wg.Done()

					l := rate.NewLimiter(500, tt.cap*8/10)

					for j := 0; j < tt.messages; j++ {
						event, ok := <-events

						if !ok {
							// on buffer overrun Watch() closes the channel
							t.Fatalf("buffer overrun")
						}

						seq, err := strconv.Atoi(event.Payload.(*machine.SequenceEvent).Sequence)
						if err != nil {
							t.Fatalf("failed to convert sequence to number: %s", err)
						}

						if seq != j {
							t.Fatalf("unexpected sequence: %d != %d", seq, j)
						}

						atomic.AddUint32(&got, 1)

						_ = l.Wait(context.Background()) //nolint:errcheck
					}
				}); err != nil {
					t.Errorf("Watch error %s", err)
				}
			}

			l := rate.NewLimiter(500, tt.cap/2)

			for i := 0; i < tt.messages; i++ {
				_ = l.Wait(context.Background()) //nolint:errcheck

				e.Publish(&machine.SequenceEvent{
					Sequence: strconv.Itoa(i),
				})
			}

			wg.Wait()

			if got != uint32(tt.messages*tt.watchers) {
				t.Errorf("Watch() = got %v, want %v", got, tt.messages*tt.watchers)
			}
		})
	}
}

func receive(t *testing.T, e runtime.Watcher, n int, opts ...runtime.WatchOptionFunc) (result []runtime.EventInfo) {
	var wg sync.WaitGroup

	wg.Add(1)

	if err := e.Watch(func(events <-chan runtime.EventInfo) {
		defer wg.Done()

		for j := 0; j < n; j++ {
			event, ok := <-events
			if !ok {
				t.Fatalf("Watch: chanel closed")
			}

			result = append(result, event)
		}

		select {
		case _, ok := <-events:
			if ok {
				t.Fatal("received extra events")
			} else {
				t.Fatalf("Watch: chanel closed")
			}
		case <-time.After(50 * time.Millisecond):
		}
	}, opts...); err != nil {
		t.Fatalf("Watch() error %s", err)
	}

	wg.Wait()

	return result
}

func extractSeq(t *testing.T, events []runtime.EventInfo) (result []int) {
	for _, event := range events {
		seq, err := strconv.Atoi(event.Payload.(*machine.SequenceEvent).Sequence)
		if err != nil {
			t.Fatalf("failed to convert sequence to number: %s", err)
		}

		result = append(result, seq)
	}

	return result
}

func gen(k, l int) (result []int) {
	for j := k; j < l; j++ {
		result = append(result, j)
	}

	return
}

func TestEvents_WatchOptionsTailEvents(t *testing.T) {
	e := NewEvents(100, 10)

	for i := 0; i < 200; i++ {
		e.Publish(&machine.SequenceEvent{
			Sequence: strconv.Itoa(i),
		})
	}

	assert.Equal(t, []int(nil), extractSeq(t, receive(t, e, 0)))
	assert.Equal(t, gen(199, 200), extractSeq(t, receive(t, e, 1, runtime.WithTailEvents(1))))
	assert.Equal(t, gen(195, 200), extractSeq(t, receive(t, e, 5, runtime.WithTailEvents(5))))
	assert.Equal(t, gen(111, 200), extractSeq(t, receive(t, e, 89, runtime.WithTailEvents(89))))
	assert.Equal(t, gen(110, 200), extractSeq(t, receive(t, e, 90, runtime.WithTailEvents(90))))
	assert.Equal(t, gen(110, 200), extractSeq(t, receive(t, e, 90, runtime.WithTailEvents(91))))   // can't tail more than cap-gap
	assert.Equal(t, gen(110, 200), extractSeq(t, receive(t, e, 90, runtime.WithTailEvents(1000)))) // can't tail more than cap-gap
	assert.Equal(t, gen(110, 200), extractSeq(t, receive(t, e, 90, runtime.WithTailEvents(-1))))   // tail all events

	e = NewEvents(100, 10)

	for i := 0; i < 30; i++ {
		e.Publish(&machine.SequenceEvent{
			Sequence: strconv.Itoa(i),
		})
	}

	assert.Equal(t, []int(nil), extractSeq(t, receive(t, e, 0)))
	assert.Equal(t, gen(29, 30), extractSeq(t, receive(t, e, 1, runtime.WithTailEvents(1))))
	assert.Equal(t, gen(28, 30), extractSeq(t, receive(t, e, 2, runtime.WithTailEvents(2))))
	assert.Equal(t, gen(25, 30), extractSeq(t, receive(t, e, 5, runtime.WithTailEvents(5))))
	assert.Equal(t, gen(0, 30), extractSeq(t, receive(t, e, 30, runtime.WithTailEvents(40))))
}

func TestEvents_WatchOptionsTailSeconds(t *testing.T) {
	e := NewEvents(100, 10)

	for i := 0; i < 20; i++ {
		e.Publish(&machine.SequenceEvent{
			Sequence: strconv.Itoa(i),
		})
	}

	// sleep to get time gap between two series of events
	time.Sleep(3 * time.Second)

	for i := 20; i < 30; i++ {
		e.Publish(&machine.SequenceEvent{
			Sequence: strconv.Itoa(i),
		})
	}

	assert.Equal(t, []int(nil), extractSeq(t, receive(t, e, 0, runtime.WithTailDuration(0))))
	assert.Equal(t, gen(20, 30), extractSeq(t, receive(t, e, 10, runtime.WithTailDuration(2*time.Second))))
	assert.Equal(t, gen(0, 30), extractSeq(t, receive(t, e, 30, runtime.WithTailDuration(10*time.Second))))
}

func TestEvents_WatchOptionsTailID(t *testing.T) {
	e := NewEvents(100, 10)

	for i := 0; i < 20; i++ {
		e.Publish(&machine.SequenceEvent{
			Sequence: strconv.Itoa(i),
		})
	}

	events := receive(t, e, 20, runtime.WithTailEvents(-1))

	for i, event := range events {
		assert.Equal(t, gen(i+1, 20), extractSeq(t, receive(t, e, 20-i-1, runtime.WithTailID(event.ID))))
	}
}

func BenchmarkWatch(b *testing.B) {
	e := NewEvents(100, 10)

	var wg sync.WaitGroup

	wg.Add(b.N)

	for i := 0; i < b.N; i++ {
		_ = e.Watch(func(events <-chan runtime.EventInfo) { wg.Done() }) //nolint:errcheck
	}

	wg.Wait()
}

func BenchmarkPublish(bb *testing.B) {
	for _, watchers := range []int{0, 1, 10} {
		bb.Run(fmt.Sprintf("Watchers-%d", watchers), func(b *testing.B) {
			e := NewEvents(10000, 10)

			var wg sync.WaitGroup

			watchers := 10

			wg.Add(watchers)

			for j := 0; j < watchers; j++ {
				_ = e.Watch(func(events <-chan runtime.EventInfo) { //nolint:errcheck
					defer wg.Done()

					for i := 0; i < b.N; i++ {
						if _, ok := <-events; !ok {
							return
						}
					}
				})
			}

			ev := machine.SequenceEvent{}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				e.Publish(&ev)
			}

			wg.Wait()
		})
	}
}
