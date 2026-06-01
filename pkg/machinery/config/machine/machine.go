// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package machine defines common machine type.
package machine

import (
	"fmt"
)

//go:generate go tool golang.org/x/tools/cmd/stringer -type=Type -linecomment

// Type represents a machine type.
type Type int

//structprotogen:gen_enum
const (
	// TypeUnknown represents undefined node type, when there is no machine configuration yet.
	TypeUnknown Type = iota // unknown

	// TypeInit type designates the first control plane node to come up. You can think of it like a bootstrap node.
	// This node will perform the initial steps to bootstrap the cluster -- generation of TLS assets, starting of the control plane, etc.
	TypeInit // init

	// TypeControlPlane designates the node as a control plane member.
	// This means it will host etcd along with the Kubernetes controlplane components such as API Server, Controller Manager, Scheduler.
	TypeControlPlane // controlplane

	// TypeWorker designates the node as a worker node.
	// This means it will be an available compute node for scheduling workloads.
	TypeWorker // worker
)

// ParseType parses string constant as Type.
func ParseType(s string) (Type, error) {
	switch s {
	case "init":
		return TypeInit, nil
	case "controlplane":
		return TypeControlPlane, nil
	case "worker", "join", "":
		return TypeWorker, nil
	case "unknown":
		return TypeUnknown, nil
	default:
		return TypeUnknown, fmt.Errorf("invalid machine type: %q", s)
	}
}

// MarshalText implements encoding.TextMarshaler.
func (t Type) MarshalText() (text []byte, err error) {
	return []byte(t.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (t *Type) UnmarshalText(text []byte) error {
	var err error

	*t, err = ParseType(string(text))

	return err
}

// IsControlPlane returns true if the type is a control plane node.
func (t Type) IsControlPlane() bool {
	return t == TypeControlPlane || t == TypeInit
}
