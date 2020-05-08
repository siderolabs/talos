// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/golang/protobuf/proto"

	"github.com/talos-systems/talos/api/machine"
)

// WatchFunc defines the watcher callback function.
type WatchFunc func(<-chan machine.Event)

// Watcher defines a runtime event watcher.
type Watcher interface {
	Watch(WatchFunc)
}

// Publisher defines a runtime event publisher.
type Publisher interface {
	Publish(proto.Message)
}

// EventStream defines the runtime event stream.
type EventStream interface {
	Watcher
	Publisher
}
