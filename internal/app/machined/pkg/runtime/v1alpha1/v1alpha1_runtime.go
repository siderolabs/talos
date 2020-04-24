// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"

// NewRuntime initializes and returns the v1alpha1 runtime.
func NewRuntime(p runtime.Platform, c runtime.Configurator, s runtime.State) *Runtime {
	return &Runtime{
		p: p,
		c: c,
		s: s,
	}
}

// Runtime implements the Runtime interface.
type Runtime struct {
	p runtime.Platform
	c runtime.Configurator
	s runtime.State
}

// Platform implements the Runtime interface.
func (d *Runtime) Platform() runtime.Platform {
	return d.p
}

// Config implements the Runtime interface.
func (d *Runtime) Config() runtime.Configurator {
	return d.c
}

// State implements the Runtime interface.
func (d *Runtime) State() runtime.State {
	return d.s
}
