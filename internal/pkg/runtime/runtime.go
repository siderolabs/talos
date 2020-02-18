// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"
	"strings"
)

// Sequence represents the sequence type.
type Sequence int

const (
	// None is the none sequence.
	None Sequence = iota
	// Boot is the boot sequence.
	Boot
	// Shutdown is the shutdown sequence.
	Shutdown
	// Upgrade is the upgrade sequence.
	Upgrade
	// Reset is the reset sequence.
	Reset
)

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

// ModeFromString returns a runtime mode that matches the given string.
func ModeFromString(s string) (m Mode, err error) {
	switch strings.Title(s) {
	case "Cloud":
		return Cloud, nil
	case "Container":
		return Container, nil
	case "Interactive":
		return Interactive, nil
	case "Metal":
		return Metal, nil
	default:
		return m, fmt.Errorf("%q is not a valid mode", s)
	}
}

// Runtime defines the runtime parameters.
type Runtime interface {
	Platform() Platform
	Config() Configurator
	Sequence() Sequence
}

// NewRuntime initializes and returns the runtime interface.
func NewRuntime(p Platform, c Configurator, s Sequence) Runtime {
	return &DefaultRuntime{
		p: p,
		c: c,
		s: s,
	}
}

// DefaultRuntime implements the Runtime interface.
type DefaultRuntime struct {
	p Platform
	c Configurator
	s Sequence
}

// Platform implements the Runtime interface.
func (d *DefaultRuntime) Platform() Platform {
	return d.p
}

// Config implements the Runtime interface.
func (d *DefaultRuntime) Config() Configurator {
	return d.c
}

// Sequence implements the Runtime interface.
func (d *DefaultRuntime) Sequence() Sequence {
	return d.s
}
