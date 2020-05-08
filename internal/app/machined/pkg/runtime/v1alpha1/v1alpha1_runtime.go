// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config"
)

// Runtime implements the Runtime interface.
type Runtime struct {
	c runtime.Configurator
	s runtime.State
	e runtime.EventStream
}

// NewRuntime initializes and returns the v1alpha1 runtime.
func NewRuntime(c runtime.Configurator, s runtime.State, e runtime.EventStream) *Runtime {
	return &Runtime{
		c: c,
		s: s,
		e: e,
	}
}

// Config implements the Runtime interface.
func (r *Runtime) Config() runtime.Configurator {
	return r.c
}

// SetConfig implements the Runtime interface.
func (r *Runtime) SetConfig(b []byte) error {
	cfg, err := config.NewFromBytes(b)
	if err != nil {
		return fmt.Errorf("failed to set config: %w", err)
	}

	r.c = cfg

	return nil
}

// State implements the Runtime interface.
func (r *Runtime) State() runtime.State {
	return r.s
}

// Events implements the Runtime interface.
func (r *Runtime) Events() runtime.EventStream {
	return r.e
}
