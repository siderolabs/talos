// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"
)

// Mode is a runtime mode.
type Mode int

// ModeCapability describes mode capability flags.
type ModeCapability uint64

const (
	// ModeCloud is the cloud runtime mode.
	ModeCloud Mode = iota
	// ModeContainer is the container runtime mode.
	ModeContainer
	// ModeMetal is the metal runtime mode.
	ModeMetal
	// ModeMetalAgent is the metal agent runtime mode.
	ModeMetalAgent
)

const (
	// Reboot node reboot.
	Reboot ModeCapability = 1 << iota
	// Rollback node rollback.
	Rollback
	// Shutdown node shutdown.
	Shutdown
	// Upgrade node upgrade.
	Upgrade
	// MetaKV is META partition.
	MetaKV
)

const (
	cloud      = "cloud"
	container  = "container"
	metal      = "metal"
	metalAgent = "metal-agent"
)

// String returns the string representation of a Mode.
func (m Mode) String() string {
	return [...]string{cloud, container, metal, metalAgent}[m]
}

// RequiresInstall implements config.RuntimeMode.
func (m Mode) RequiresInstall() bool {
	return m == ModeMetal
}

// InContainer implements config.RuntimeMode.
func (m Mode) InContainer() bool {
	return m == ModeContainer
}

// Supports returns mode capability.
func (m Mode) Supports(feature ModeCapability) bool {
	return (m.capabilities() & uint64(feature)) != 0
}

// IsAgent returns true if the mode is an agent mode (i.e. metal agent mode).
func (m Mode) IsAgent() bool {
	return m == ModeMetalAgent
}

// ParseMode returns a `Mode` that matches the specified string.
func ParseMode(s string) (mod Mode, err error) {
	switch s {
	case cloud:
		mod = ModeCloud
	case container:
		mod = ModeContainer
	case metal:
		mod = ModeMetal
	case metalAgent:
		mod = ModeMetalAgent
	default:
		return mod, fmt.Errorf("unknown runtime mode: %q", s)
	}

	return mod, nil
}

func (m Mode) capabilities() uint64 {
	all := ^uint64(0)

	return [...]uint64{
		// metal
		all,
		// container
		all ^ uint64(Reboot|Shutdown|Upgrade|Rollback|MetaKV),
		// cloud
		all,
	}[m]
}
