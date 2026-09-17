// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build sidero.debug

package ctrltrace

import (
	"context"
	"sync"

	"github.com/cosi-project/runtime/pkg/controller"
	"go.uber.org/zap"
)

// WrapController wraps a controller to record its lifecycle and to attribute
// every state operation it performs to its name.
//
// When tracing is off the controller is returned unchanged.
func WrapController(ctrl controller.Controller) controller.Controller {
	t := Global()
	if t == nil {
		return ctrl
	}

	return &tracedController{Controller: ctrl, t: t}
}

type tracedController struct {
	controller.Controller

	t *Tracer

	mu      sync.Mutex
	inCtx   context.Context //nolint:containedctx
	inRt    controller.Runtime
	tagged  context.Context //nolint:containedctx
	wrapped *tracedRuntime
}

// Run wraps the controller's Run method.
//
// The controller runtime restarts a failed controller by calling Run again with the very
// same context and runtime values, and controllers are allowed to rely on that identity
// (internal/pkg/dns.Manager.ServeBackground panics if it is handed a different context on a
// restart). So the tagged context and the runtime wrapper are built once per (ctx, runtime)
// pair and reused — which also keeps a restart from leaving a second event relay behind,
// competing with the first one for the same upstream channel.
func (c *tracedController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	name := c.Controller.Name()

	c.mu.Lock()

	if c.inCtx != ctx || c.inRt != r {
		c.inCtx, c.inRt = ctx, r
		// the caller tag rides along the context into every Get/List/Modify the controller makes
		c.tagged = WithCaller(ctx, name)
		c.wrapped = &tracedRuntime{Runtime: r, t: c.t, name: name, ctx: c.tagged}
	}

	tagged, wrapped := c.tagged, c.wrapped

	c.mu.Unlock()

	c.t.Ctrl(name, "start", nil)

	err := c.Controller.Run(tagged, wrapped, logger)

	if err != nil {
		c.t.Ctrl(name, "crash", err)
	} else {
		c.t.Ctrl(name, "stop", nil)
	}

	return err
}

// tracedRuntime intercepts the reconcile events delivered to the controller.
//
// Reads and writes need no interception here: they are recorded by the state wrapper,
// attributed through the context.
type tracedRuntime struct {
	controller.Runtime

	t    *Tracer
	name string
	ctx  context.Context //nolint:containedctx

	once sync.Once
	ch   chan controller.ReconcileEvent
}

// EventCh relays reconcile events, recording the exact moment the controller picks one up.
//
// The relay is an unbuffered hand-off, so a send completing means the controller was idle
// and has just entered a new reconcile cycle. Where that cycle ends is reconstructed offline
// from the controller's last state operation before the next wake-up.
func (r *tracedRuntime) EventCh() <-chan controller.ReconcileEvent {
	r.once.Do(func() {
		r.ch = make(chan controller.ReconcileEvent)

		go r.relay()
	})

	return r.ch
}

func (r *tracedRuntime) relay() {
	upstream := r.Runtime.EventCh()

	for {
		select {
		case <-r.ctx.Done():
			return
		case ev, ok := <-upstream:
			if !ok {
				close(r.ch)

				return
			}

			select {
			case <-r.ctx.Done():
				return
			case r.ch <- ev:
				r.t.Ctrl(r.name, "wake", nil)
			}
		}
	}
}

func (r *tracedRuntime) QueueReconcile() {
	r.t.Ctrl(r.name, "queue", nil)

	r.Runtime.QueueReconcile()
}

func (r *tracedRuntime) UpdateInputs(inputs []controller.Input) error {
	err := r.Runtime.UpdateInputs(inputs)

	ev := Event{K: "ctrl", Op: "inputs", Ctrl: r.name, N: len(inputs)}
	if err != nil {
		ev.Err = err.Error()
	}

	r.t.Emit(ev)

	return err
}
