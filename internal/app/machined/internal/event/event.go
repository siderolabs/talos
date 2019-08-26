/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package event implements an embeddable type that uses the observation
// pattern to facilitate an event bus.
package event

// Type is event type.
type Type int

const (
	// Shutdown is the shutdown event.
	Shutdown = Type(iota)
	// Reboot is the reboot event.
	Reboot
	// Upgrade is the upgrade event.
	Upgrade
)

// Event represents an event in the observer pattern.
type Event struct {
	Type Type
	Data interface{}
}

// Channel is a channel for sending events.
type Channel chan Event

// Listeners is a slice of listeners to send events to.
type Listeners []Channel

// Observer is a component of the observer design pattern.
type Observer interface {
	Channel() Channel
	Types() []Type
}

// Notifier is a component of the observer design pattern.
type Notifier interface {
	Notify(Event)
	Register(Observer, ...Type)
	Unregister(Observer)
}

// ObserveNotifier is a composite interface consisting of the Observer, and
// Notifier interfaces.
type ObserveNotifier interface {
	Observer
	Notifier
}

// Embeddable is a type that implements sane defaults as an observer.
type Embeddable struct {
	channel Channel
	types   []Type
}

// Channel implements the Observer interface.
func (e *Embeddable) Channel() Channel {
	if cap(e.channel) == 0 {
		e.channel = make(Channel, 20)
	}
	return e.channel
}

// Types implements the Observer interface.
func (e *Embeddable) Types() []Type {
	if e.types == nil {
		e.types = []Type{Shutdown, Reboot, Upgrade}
	}
	return e.types
}
