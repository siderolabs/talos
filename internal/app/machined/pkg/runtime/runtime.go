// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

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
