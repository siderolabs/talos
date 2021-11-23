// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"sync"
)

// NewDrainer creates new drainer.
func NewDrainer() *Drainer {
	return &Drainer{
		shutdown: make(chan struct{}, 1),
	}
}

// Drainer is used in controllers to ensure graceful shutdown.
type Drainer struct {
	subscriptionsMu sync.Mutex
	subscriptions   []*DrainSubscription
	shutdown        chan struct{}
}

// Drain initializes drain sequence waits for it to succeed until the context is canceled.
func (d *Drainer) Drain(ctx context.Context) error {
	d.subscriptionsMu.Lock()
	for _, s := range d.subscriptions {
		select {
		case s.events <- DrainEvent{}:
		default:
		}
	}
	d.subscriptionsMu.Unlock()

	for {
		select {
		case <-d.shutdown:
			d.subscriptionsMu.Lock()
			l := len(d.subscriptions)
			d.subscriptionsMu.Unlock()

			if l == 0 {
				return nil
			}
		case <-ctx.Done():
			return context.Canceled
		}
	}
}

// Subscribe should be called from a controller that needs graceful shutdown.
func (d *Drainer) Subscribe() *DrainSubscription {
	d.subscriptionsMu.Lock()
	defer d.subscriptionsMu.Unlock()

	subscription := &DrainSubscription{
		events:  make(chan DrainEvent, 1),
		drainer: d,
	}

	d.subscriptions = append(d.subscriptions, subscription)

	return subscription
}

// DrainSubscription keeps ingoing and outgoing events channels.
type DrainSubscription struct {
	drainer *Drainer
	events  chan DrainEvent
}

// EventCh returns drain events channel.
func (s *DrainSubscription) EventCh() <-chan DrainEvent {
	return s.events
}

// Cancel the subscription which triggers drain to shutdown.
func (s *DrainSubscription) Cancel() {
	s.drainer.subscriptionsMu.Lock()
	defer s.drainer.subscriptionsMu.Unlock()

	for i, sub := range s.drainer.subscriptions {
		if sub == s {
			s.drainer.subscriptions = append(s.drainer.subscriptions[:i], s.drainer.subscriptions[i+1:]...)

			break
		}
	}

	select {
	case s.drainer.shutdown <- struct{}{}:
	default:
	}
}

// DrainEvent is sent to the events channel when drainer starts the shutdown sequence.
type DrainEvent struct{}
