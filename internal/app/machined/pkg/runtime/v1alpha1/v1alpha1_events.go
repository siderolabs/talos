// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

// Events represents the runtime event stream.
type Events struct{}

// NewEvents initializes and returns the v1alpha1 runtime.
func NewEvents() *Events {
	return &Events{}
}

// Watch implements the Events interface.
func (r *Events) Watch(f func(<-chan runtime.Event)) {
}

// Publish implements the Events interface.
func (r *Events) Publish(runtime.Event) {
}
