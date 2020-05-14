// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: scopelint
package v1alpha1

import (
	"reflect"
	"sync"
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/talos-systems/talos/api/machine"
)

func TestNewEvents(t *testing.T) {
	type args struct {
		n int
	}

	tests := []struct {
		name string
		args args
		want *Events
	}{
		{
			name: "success",
			args: args{
				n: 100,
			},
			want: &Events{
				subscribers: make([]chan machine.Event, 0, 100),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewEvents(tt.args.n); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewEvents() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkWatch(b *testing.B) {
	e := NewEvents(b.N)

	var wg sync.WaitGroup

	wg.Add(b.N)

	for i := 0; i < b.N; i++ {
		e.Watch(func(events <-chan machine.Event) { wg.Done() })
	}

	wg.Wait()
}

func TestEvents_Watch(t *testing.T) {
	type fields struct {
		subscribers []chan machine.Event
	}

	tests := []struct {
		name   string
		count  int
		fields fields
	}{
		{
			name:  "success",
			count: 10,
			fields: fields{
				subscribers: make([]chan machine.Event, 0, 100),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Events{
				subscribers: tt.fields.subscribers,
			}

			var wg sync.WaitGroup
			wg.Add(tt.count)

			for i := 0; i < tt.count; i++ {
				e.Watch(func(events <-chan machine.Event) { wg.Done() })
			}

			wg.Wait()

			// We need to lock here to prevent a race condition when checking the
			// number of subscribers.
			e.Lock()
			defer e.Unlock()

			// We can only check if the number of subscribers decreases because there
			// is a race condition between the tear down of subscriber and the above
			// lock. In other words, there is a chance that the number of subscribers
			// is not zero.
			if len(e.subscribers) > tt.count {
				t.Errorf("Watch() = got %v subscribers, expected to be < %v", len(e.subscribers), tt.count)
			}
		})
	}
}

func TestEvents_Publish(t *testing.T) {
	type fields struct {
		subscribers []chan machine.Event
		Mutex       *sync.Mutex
	}

	type args struct {
		event proto.Message
	}

	tests := []struct {
		name   string
		count  int
		fields fields
		args   args
	}{
		{
			name:  "success",
			count: 10,
			fields: fields{
				subscribers: make([]chan machine.Event, 0, 100),
				Mutex:       &sync.Mutex{},
			},
			args: args{
				event: &machine.SequenceEvent{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Events{
				subscribers: tt.fields.subscribers,
			}

			var wg sync.WaitGroup
			wg.Add(tt.count)

			mu := &sync.Mutex{}

			got := 0

			for i := 0; i < tt.count; i++ {
				e.Watch(func(events <-chan machine.Event) {
					<-events

					mu.Lock()
					got++
					mu.Unlock()

					wg.Done()
				})
			}

			e.Publish(tt.args.event)

			wg.Wait()

			if got != tt.count {
				t.Errorf("Watch() = got %v, want %v", got, tt.count)
			}
		})
	}
}

func BenchmarkPublish(b *testing.B) {
	e := NewEvents(b.N)

	var wg sync.WaitGroup

	wg.Add(b.N)

	for i := 0; i < b.N; i++ {
		e.Watch(func(events <-chan machine.Event) {
			<-events

			wg.Done()
		})
	}

	e.Publish(&machine.SequenceEvent{})

	wg.Wait()
}
