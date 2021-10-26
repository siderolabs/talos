// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=Scope -linecomment

// Scope is an address scope.
type Scope uint8

// MarshalYAML implements yaml.Marshaler.
func (scope Scope) MarshalYAML() (interface{}, error) {
	return scope.String(), nil
}

// Scope constants.
const (
	ScopeGlobal  Scope = 0   // global
	ScopeSite    Scope = 200 // site
	ScopeLink    Scope = 253 // link
	ScopeHost    Scope = 254 // host
	ScopeNowhere Scope = 255 // nowhere
)
