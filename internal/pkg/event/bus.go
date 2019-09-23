/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package event

import (
	"sync"
)

type singleton struct {
	listeners map[Type]Listeners
	mu        sync.RWMutex
}

var (
	bus  *singleton
	once sync.Once
)

// Bus represents an event bus.
//
// nolint: golint
func Bus() *singleton {
	once.Do(func() {
		bus = &singleton{}
	})

	return bus
}

// Notify implements the Notifier interface.
func (s *singleton) Notify(e Event) {
	s.mu.RLock()
	listeners := append(Listeners{}, s.listeners[e.Type]...)
	s.mu.RUnlock()

	for _, c := range listeners {
		c <- e
	}
}

// Register implements the Notifier interface.
func (s *singleton) Register(o Observer, types ...Type) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listeners == nil {
		s.listeners = make(map[Type]Listeners)
	}
	for _, t := range o.Types() {
		s.listeners[t] = append(s.listeners[t], o.Channel())
	}
}

// Unregister implements the Notifier interface.
func (s *singleton) Unregister(o Observer, types ...Type) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range o.Types() {
		for i := 0; i < len(s.listeners[t]); {
			if s.listeners[t][i] == o.Channel() {
				s.listeners[t][i] = s.listeners[t][len(s.listeners[t])-1]
				s.listeners[t][len(s.listeners[t])-1] = nil
				s.listeners[t] = s.listeners[t][:len(s.listeners[t])-1]
			} else {
				i++
			}
		}
	}
}
