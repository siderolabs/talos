// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

type Event struct {
	Data interface{}
}

type Watcher interface {
	Watch(func(<-chan Event))
}

type Publisher interface {
	Publish(Event)
}

// EventStream
type EventStream interface {
	Watcher
	Publisher
}
