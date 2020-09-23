// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
)

// Runtime implements the Runtime interface.
type Runtime struct {
	c config.Provider
	s runtime.State
	e runtime.EventStream
	l runtime.LoggingManager
}

// NewRuntime initializes and returns the v1alpha1 runtime.
func NewRuntime(c config.Provider, s runtime.State, e runtime.EventStream, l runtime.LoggingManager) *Runtime {
	return &Runtime{
		c: c,
		s: s,
		e: e,
		l: l,
	}
}

// Config implements the Runtime interface.
func (r *Runtime) Config() config.Provider {
	return r.c
}

// SetConfig implements the Runtime interface.
func (r *Runtime) SetConfig(b []byte) error {
	cfg, err := configloader.NewFromBytes(b)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(r.State().Platform().Mode()); err != nil {
		return fmt.Errorf("failed to validate config: %w", err)
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

// Logging implements the Runtime interface.
func (r *Runtime) Logging() runtime.LoggingManager {
	return r.l
}
