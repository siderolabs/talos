// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: scopelint
package v1alpha1

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/talos-systems/talos/api/machine"
	"golang.org/x/time/rate"
)

func BenchmarkWatch(b *testing.B) {
	e := NewEvents(100)

	var wg sync.WaitGroup

	wg.Add(b.N)

	for i := 0; i < b.N; i++ {
		e.Watch(func(events <-chan machine.Event) { wg.Done() })
	}

	wg.Wait()
}

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := NewEvents(tt.cap)

			var wg sync.WaitGroup
			wg.Add(tt.watchers)

			got := uint32(0)

			for i := 0; i < tt.watchers; i++ {
				e.Watch(func(events <-chan machine.Event) {
					defer wg.Done()

					for j := 0; j < tt.messages; j++ {
						event := <-events

						var msg machine.SequenceEvent

						if err := proto.Unmarshal(event.GetData().GetValue(), &msg); err != nil {
							t.Fatalf("failed to unmarshal message: %s", err)
						}

						seq, err := strconv.Atoi(msg.Sequence)
						if err != nil {
							t.Fatalf("failed to convert sequence to number: %s", err)
						}

						if seq != j {
							t.Fatalf("unexpected sequence: %d != %d", seq, j)
						}

						atomic.AddUint32(&got, 1)
					}
				})
			}

			l := rate.NewLimiter(1000, tt.cap/2)

			for i := 0; i < tt.messages; i++ {
				_ = l.Wait(context.Background())

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

func BenchmarkPublish(b *testing.B) {
	e := NewEvents(10000)

	var wg sync.WaitGroup

	watchers := 10

	wg.Add(watchers)

	for j := 0; j < watchers; j++ {
		e.Watch(func(events <-chan machine.Event) {
			defer wg.Done()

			for i := 0; i < b.N; i++ {
				if _, ok := <-events; !ok {
					return
				}
			}
		})
	}

	ev := machine.SequenceEvent{}

	for i := 0; i < b.N; i++ {
		e.Publish(&ev)
	}

	wg.Wait()
}
