// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package machine

import "fmt"

// Type represents a machine type.
type Type int

const (
	// TypeUnknown represents undefined node type.
	TypeUnknown Type = iota
	// TypeInit represents a bootstrap node.
	TypeInit
	// TypeControlPlane represents a control plane node.
	TypeControlPlane
	// TypeJoin represents a worker node.
	TypeJoin
)

const (
	typeUnknown      = "unknown"
	typeInit         = "init"
	typeControlPlane = "controlplane"
	typeJoin         = "join"
)

// String returns the string representation of Type.
func (t Type) String() string {
	return [...]string{typeUnknown, typeInit, typeControlPlane, typeJoin}[t]
}

// ParseType parses string constant as Type.
func ParseType(t string) (Type, error) {
	switch t {
	case typeUnknown:
		return TypeUnknown, nil
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
