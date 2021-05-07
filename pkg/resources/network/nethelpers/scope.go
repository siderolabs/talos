// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "golang.org/x/sys/unix"

//go:generate stringer -type=Scope -linecomment

// Scope is an address scope.
type Scope uint8

// MarshalYAML implements yaml.Marshaler.
func (scope Scope) MarshalYAML() (interface{}, error) {
	return scope.String(), nil
}

// Scope constants.
const (
	ScopeGlobal  Scope = unix.RT_SCOPE_UNIVERSE // global
	ScopeSite    Scope = unix.RT_SCOPE_SITE     // site
	ScopeLink    Scope = unix.RT_SCOPE_LINK     // link
	ScopeHost    Scope = unix.RT_SCOPE_HOST     // host
	ScopeNowhere Scope = unix.RT_SCOPE_NOWHERE  // nowhere
)
