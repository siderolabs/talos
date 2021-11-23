// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

// NewDrainer creates new drainer.
func NewDrainer() *Drainer {
	return &Drainer{}
}

// Drainer is used in controllers to ensure graceful shutdown.
type Drainer struct {
	subscriptionsMu sync.Mutex
	subscriptions   []*DrainSubscription
}

// Drain initializes drain sequence waits for it to succeed until the context is canceled.
func (d *Drainer) Drain(ctx context.Context) error {
	var eg errgroup.Group

	d.subscriptionsMu.Lock()
	defer d.subscriptionsMu.Unlock()

	for _, s := range d.subscriptions {
		s := s

		eg.Go(func() error {
			select {
			case s.events <- DrainEvent{}:
			case <-ctx.Done():
				return context.Canceled
			}

			return nil
		})

		eg.Go(func() error {
			select {
			case <-s.shutdown:
			case <-ctx.Done():
				return context.Canceled
			}

			return nil
		})
	}

	return eg.Wait()
}

// Subscribe should be called from a controller that needs graceful shutdown.
func (d *Drainer) Subscribe() *DrainSubscription {
	d.subscriptionsMu.Lock()
	defer d.subscriptionsMu.Unlock()

	subscription := &DrainSubscription{
		events:   make(chan DrainEvent),
		shutdown: make(chan struct{}),
	}

	d.subscriptions = append(d.subscriptions, subscription)

	return subscription
}

// DrainSubscription keeps ingoing and outgoing events channels.
type DrainSubscription struct {
	events   chan DrainEvent
	shutdown chan struct{}
}

// EventCh returns drain events channel.
func (s *DrainSubscription) EventCh() <-chan DrainEvent {
	return s.events
}

// Cancel the subscription which triggers drain to shutdown.
func (s *DrainSubscription) Cancel() {
	select {
	case s.shutdown <- struct{}{}:
	default:
	}
}

// DrainEvent is sent to the events channel when drainer starts the shutdown sequence.
type DrainEvent struct{}
