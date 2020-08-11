// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package vm implements common methods for VM provisioners.
package vm

const stateFileName = "state.yaml"

// Provisioner base for VM provisioners.
type Provisioner struct {
	// Name actual provisioner type.
	Name string
}
