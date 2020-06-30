// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

// Pod presents information about a pod, including a list of containers.
type Pod struct {
	Name    string
	Sandbox string

	Containers []*Container
}
