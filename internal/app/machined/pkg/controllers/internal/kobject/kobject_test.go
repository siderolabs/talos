// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kobject_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/kobject"
)

func TestWatcher(t *testing.T) {
	watcher, err := kobject.NewWatcher(zaptest.NewLogger(t))
	require.NoError(t, err)

	evCh := watcher.Run("mock")

	require.NoError(t, watcher.Close())

	// the evCh should be closed
	for range evCh { //nolint:revive
	}
}

type fakeReceiver struct {
	events chan *kobject.Event
	once   sync.Once
}

func newFakeReceiver(events ...*kobject.Event) *fakeReceiver {
	f := &fakeReceiver{
		events: make(chan *kobject.Event, len(events)),
	}

	for _, ev := range events {
		f.events <- ev
	}

	return f
}

func (f *fakeReceiver) Receive() (*kobject.Event, error) {
	ev, ok := <-f.events
	if !ok {
		return nil, errors.New("use of closed file")
	}

	return ev, nil
}

func (f *fakeReceiver) Close() error {
	f.once.Do(func() { close(f.events) })

	return nil
}

func TestWatcherSubsystemFilter(t *testing.T) {
	for _, tc := range []struct {
		name      string
		events    []*kobject.Event
		subsystem string
		wantCount int
	}{
		{
			name: "filters mixed subsystems",
			events: []*kobject.Event{
				{Subsystem: "module"},
				{Subsystem: "test"},
				{Subsystem: "block"},
				{Subsystem: "test"},
				{Subsystem: "test"},
			},
			subsystem: "test",
			wantCount: 3,
		},
		{
			name: "no matching events",
			events: []*kobject.Event{
				{Subsystem: "module"},
				{Subsystem: "block"},
			},
			subsystem: "test",
			wantCount: 0,
		},
		{
			name: "all events match",
			events: []*kobject.Event{
				{Subsystem: "test"},
				{Subsystem: "test"},
			},
			subsystem: "test",
			wantCount: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeReceiver(tc.events...)

			watcher := kobject.NewWatcherFromReceiver(fake, zaptest.NewLogger(t))
			evCh := watcher.Run(tc.subsystem)

			require.NoError(t, watcher.Close())

			var received []*kobject.Event

			for ev := range evCh {
				received = append(received, ev)
			}

			require.Len(t, received, tc.wantCount)

			for _, ev := range received {
				require.Equal(t, tc.subsystem, ev.Subsystem)
			}
		})
	}
}
