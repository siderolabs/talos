// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !sidero.debug

// Package ctrltrace implements a debug-only tracer for the COSI controller runtime.
//
// This is the production build: the tracer is compiled in only with `WITH_DEBUG=1`
// (build tag `sidero.debug`), so everything here is inert and the real implementation —
// which can write resource specs, secrets included, to a file — is not in the binary at all.
package ctrltrace

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/state"
)

// Tracer is not available in this build.
type Tracer struct{}

// Enabled always reports false without the `sidero.debug` build tag.
func Enabled() bool { return false }

// Global returns nil without the `sidero.debug` build tag.
func Global() *Tracer { return nil }

// Setup does nothing without the `sidero.debug` build tag.
func Setup(string) *Tracer { return nil }

// Close does nothing.
func (*Tracer) Close() error { return nil }

// WrapState returns the state unchanged.
func WrapState(inner state.CoreState) state.CoreState { return inner }

// WrapController returns the controller unchanged.
func WrapController(ctrl controller.Controller) controller.Controller { return ctrl }

// EmitGraph does nothing.
func EmitGraph(*controller.DependencyGraph) {}

// WithCaller returns the context unchanged.
func WithCaller(ctx context.Context, _ string) context.Context { return ctx }

// CallerFrom returns an empty string.
func CallerFrom(context.Context) string { return "" }
