// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !sidero.debug

package ctrltrace_test

import (
	"context"
	"testing"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/ctrltrace"
)

type noopController struct{}

func (noopController) Name() string                 { return "test.NoopController" }
func (noopController) Inputs() []controller.Input   { return nil }
func (noopController) Outputs() []controller.Output { return nil }

func (noopController) Run(context.Context, controller.Runtime, *zap.Logger) error { return nil }

// TestProductionBuildIsInert: without `sidero.debug` the tracer must not exist at all —
// no wrapping of the resource state, no wrapping of the controllers, nothing to turn on.
func TestProductionBuildIsInert(t *testing.T) {
	assert.False(t, ctrltrace.Enabled())
	assert.Nil(t, ctrltrace.Global())
	assert.Nil(t, ctrltrace.Setup("v0.0.0-test"))
	assert.NoError(t, ctrltrace.Setup("v0.0.0-test").Close())

	st := namespaced.NewState(inmem.Build)
	assert.Same(t, st, ctrltrace.WrapState(st))

	ctrl := noopController{}
	assert.Equal(t, controller.Controller(ctrl), ctrltrace.WrapController(ctrl))

	assert.Empty(t, ctrltrace.CallerFrom(ctrltrace.WithCaller(t.Context(), "test.Controller")))

	_ = ctrltrace.WrapState(st)
}
