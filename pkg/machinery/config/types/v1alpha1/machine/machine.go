// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package machine

import "fmt"

// Type represents a machine type.
type Type int

const (
	// TypeInit represents a bootstrap node.
	TypeInit Type = iota
	// TypeControlPlane represents a control plane node.
	TypeControlPlane
	// TypeJoin represents a worker node.
	TypeJoin
)

const (
	typeInit         = "init"
	typeControlPlane = "controlplane"
	typeJoin         = "join"
)

// String returns the string representation of Type.
func (t Type) String() string {
	return [...]string{typeInit, typeControlPlane, typeJoin}[t]
}

// ParseType parses string constant as Type.
func ParseType(t string) (Type, error) {
	switch t {
	case typeInit:
		return TypeInit, nil
	case typeControlPlane:
		return TypeControlPlane, nil
	case typeJoin:
		return TypeJoin, nil
	default:
		return 0, fmt.Errorf("unknown machine type: %q", t)
	}
}
