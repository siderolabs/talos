/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package event

import "sync"

// Subscriber is a channel used to receive events from the bus
type Subscriber chan<- Type

type singleton struct {
	mu          sync.RWMutex
	subscribers []Subscriber
}

var (
	bus  *singleton
	once sync.Once
)

// Bus represents a singletone middleware which acts a proxy between event publishers and subscribers.
//
//nolint: golint
func Bus() *singleton {
	once.Do(func() {
		bus = &singleton{}
	})

	return bus
}

func (s *singleton) Publish(e Type) {
	s.mu.RLock()
	subscribers := append([]Subscriber{}, s.subscribers...)
	s.mu.RUnlock()

	for _, subscriber := range subscribers {
		subscriber <- e
	}
}

func (s *singleton) Subscribe(subsciber Subscriber) {
	s.mu.Lock()
	s.subscribers = append(s.subscribers, subsciber)
	s.mu.Unlock()
}

func (s *singleton) Unsubscribe(subscriber Subscriber) {
	s.mu.Lock()
	for i := 0; i < len(s.subscribers); {
		if s.subscribers[i] == subscriber {
			s.subscribers[i] = s.subscribers[len(s.subscribers)-1]
			s.subscribers[len(s.subscribers)-1] = nil
			s.subscribers = s.subscribers[:len(s.subscribers)-1]
		} else {
			i++
		}
	}
	s.mu.Unlock()
}
