// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import "syscall"

// Inspector gather information about pods & containers.
type Inspector interface {
	// Pods collects information about running pods & containers.
	Pods() ([]*Pod, error)
	// Container returns info about a single container.
	Container(id string) (*Container, error)
	// Close frees associated resources.
	Close() error
	// Returns path to the container's stderr pipe
	GetProcessStderr(ID string) (string, error)
	// Kill sends signal to container's process
	Kill(ID string, isPodSandbox bool, signal syscall.Signal) error
}
