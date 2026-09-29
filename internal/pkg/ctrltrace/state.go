// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build sidero.debug

package ctrltrace

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.yaml.in/yaml/v4"
)

// tracedState wraps state.CoreState recording every operation which goes through it.
type tracedState struct {
	inner state.CoreState
	t     *Tracer
}

// WrapState wraps the core state with the tracer, if tracing is enabled.
//
// When tracing is off the state is returned unchanged, so there is no overhead at all.
func WrapState(inner state.CoreState) state.CoreState {
	t := Global()
	if t == nil {
		return inner
	}

	return &tracedState{inner: inner, t: t}
}

func (s *tracedState) event(ctx context.Context, op string, ptr resource.Pointer, started time.Time, err error) Event {
	ev := Event{
		K:    "op",
		Op:   op,
		Ctrl: CallerFrom(ctx),
		Dur:  time.Since(started).Microseconds(),
	}

	if ptr != nil {
		ev.NS, ev.RT, ev.ID = ptr.Namespace(), ptr.Type(), ptr.ID()
	}

	if err != nil {
		ev.Err = err.Error()
	}

	return ev
}

func (s *tracedState) Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) {
	started := time.Now()

	r, err := s.inner.Get(ctx, ptr, opts...)

	// reads outside of a controller (API server, sequencer) are not interesting, and there are many of them
	if name := CallerFrom(ctx); name != "" {
		ev := s.event(ctx, "get", ptr, started, err)

		if r != nil {
			fillMeta(&ev, r)
		}

		s.t.Emit(ev)
	}

	return r, err
}

func (s *tracedState) List(ctx context.Context, kind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	started := time.Now()

	list, err := s.inner.List(ctx, kind, opts...)

	if name := CallerFrom(ctx); name != "" {
		ev := s.event(ctx, "list", nil, started, err)
		ev.NS, ev.RT = kind.Namespace(), kind.Type()
		ev.N = len(list.Items)

		s.t.Emit(ev)
	}

	return list, err
}

func (s *tracedState) Create(ctx context.Context, r resource.Resource, opts ...state.CreateOption) error {
	started := time.Now()

	err := s.inner.Create(ctx, r, opts...)

	ev := s.event(ctx, "create", r.Metadata(), started, err)
	fillMeta(&ev, r)
	s.fillSpec(&ev, r)

	s.t.Emit(ev)

	return err
}

func (s *tracedState) Update(ctx context.Context, r resource.Resource, opts ...state.UpdateOption) error {
	started := time.Now()

	err := s.inner.Update(ctx, r, opts...)

	ev := s.event(ctx, "update", r.Metadata(), started, err)
	fillMeta(&ev, r)
	s.fillSpec(&ev, r)

	s.t.Emit(ev)

	return err
}

func (s *tracedState) Destroy(ctx context.Context, ptr resource.Pointer, opts ...state.DestroyOption) error {
	started := time.Now()

	err := s.inner.Destroy(ctx, ptr, opts...)

	s.t.Emit(s.event(ctx, "destroy", ptr, started, err))

	return err
}

func (s *tracedState) Watch(ctx context.Context, ptr resource.Pointer, ch chan<- state.Event, opts ...state.WatchOption) error {
	started := time.Now()

	err := s.inner.Watch(ctx, ptr, ch, opts...)

	s.t.Emit(s.event(ctx, "watch", ptr, started, err))

	return err
}

func (s *tracedState) WatchKind(ctx context.Context, kind resource.Kind, ch chan<- state.Event, opts ...state.WatchKindOption) error {
	started := time.Now()

	err := s.inner.WatchKind(ctx, kind, ch, opts...)

	ev := s.event(ctx, "watchkind", nil, started, err)
	ev.NS, ev.RT = kind.Namespace(), kind.Type()

	s.t.Emit(ev)

	return err
}

func (s *tracedState) WatchKindAggregated(ctx context.Context, kind resource.Kind, ch chan<- []state.Event, opts ...state.WatchKindOption) error {
	started := time.Now()

	err := s.inner.WatchKindAggregated(ctx, kind, ch, opts...)

	ev := s.event(ctx, "watchkind", nil, started, err)
	ev.NS, ev.RT = kind.Namespace(), kind.Type()

	s.t.Emit(ev)

	return err
}

// fillSpec records the resource spec, secrets included.
//
// The API redacts resources Talos marks sensitive; the tracer deliberately does not. It only
// exists in a `WITH_DEBUG=1` build pointed at a throwaway cluster, and a secret whose contents
// you cannot see is not much use when you are debugging how it got there. The trace file is
// still written 0600, and is still not something to hand around.
func (s *tracedState) fillSpec(ev *Event, r resource.Resource) {
	if !s.t.withSpecs {
		return
	}

	out, err := yaml.Marshal(r.Spec())
	if err != nil {
		return
	}

	const maxSpec = 8 << 10

	if len(out) > maxSpec {
		out = append(out[:maxSpec:maxSpec], "\n# ... truncated\n"...)
	}

	ev.Spec = string(out)
}

func fillMeta(ev *Event, r resource.Resource) {
	md := r.Metadata()

	ev.Owner = md.Owner()
	ev.Ver = md.Version().String()
	ev.Phase = md.Phase().String()

	if fin := md.Finalizers(); fin != nil && len(*fin) > 0 {
		ev.Fin = append([]string(nil), *fin...)
	}
}
