/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package runtime

import (
	"github.com/talos-systems/talos/pkg/config"
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

// Runtime defines the runtime parameters.
type Runtime interface {
	Platform() Platform
	Config() config.Configurator
}

// NewRuntime initializes and returns the runtime interface.
func NewRuntime(p Platform, c config.Configurator) Runtime {
	return &DefaultRuntime{
		p: p,
		c: c,
	}
}

// DefaultRuntime implements the Runtime interface.
type DefaultRuntime struct {
	p Platform
	c config.Configurator
}

// Platform implements the Runtime interface.
func (d *DefaultRuntime) Platform() Platform {
	return d.p
}

// Config implements the Runtime interface.
func (d *DefaultRuntime) Config() config.Configurator {
	return d.c
}
