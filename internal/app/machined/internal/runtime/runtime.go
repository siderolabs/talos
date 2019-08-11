/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package runtime

// Mode is a runtime mode.
type Mode int

const (
	// Cloud represents a runtime mode.
	Cloud Mode = iota
	// Container represents a runtime mode.
	Container
	// Interactive represents a runtime mode.
	Interactive
	// Metal represents a runtime mode.
	Metal
)

// String returns the string representation of a Mode.
func (m Mode) String() string {
	return [...]string{"Cloud", "Container", "Interactive", "Metal"}[m]
}
