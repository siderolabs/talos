// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// Container is an initializer that is a noop.
type Container struct{}

// Initialize implements the Initializer interface.
func (c *Container) Initialize(r runtime.Runtime) (err error) {
	return nil
}
