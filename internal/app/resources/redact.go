// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resources

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/grpc/middleware/authz"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// Redact wraps the state, redacting the sensitive values of the resources on the way out.
//
// Only the resource types registered with [redactable.Register] are ever redacted, and those are
// always sensitive; every other type is served as it is. Whether a read of a redactable type is
// redacted follows from the redaction mode the requester asked for with the [redactable.Header] and
// the roles it carries:
//
//   - no header: nothing is redacted, which is what a client predating redaction gets;
//   - "accept": an os:admin is served in full, anyone else is served redacted;
//   - "enforce": everything is redacted, including for an os:admin.
//
// Only the spec fields marked with the `redact:` struct tag are replaced, and only for the specs
// implementing [redactable.Spec]; a resource which actually lost something is marked with the
// [constants.AnnotationRedacted] annotation, so that the client can tell a redacted value apart from
// one genuinely set to the placeholder. Nothing else in the metadata is touched: IDs, labels and the
// rest of the annotations are returned as they are.
//
// Redaction is not a substitute for the access policy: see [AccessPolicy] for the decision on whether
// the resource may be read at all, and [redactable.Header] for the two together.
func Redact(st state.CoreState) state.CoreState { //nolint:ireturn
	return &redactingState{state: st}
}

type redactingState struct {
	state state.CoreState
}

// Get implements state.CoreState interface.
func (r *redactingState) Get(ctx context.Context, ptr resource.Pointer, opts ...state.GetOption) (resource.Resource, error) { //nolint:ireturn
	res, err := r.state.Get(ctx, ptr, opts...)
	if err != nil {
		return nil, err
	}

	if !redactionRequired(ctx, ptr.Type()) {
		return res, nil
	}

	return redactResource(res), nil
}

// List implements state.CoreState interface.
func (r *redactingState) List(ctx context.Context, kind resource.Kind, opts ...state.ListOption) (resource.List, error) {
	list, err := r.state.List(ctx, kind, opts...)
	if err != nil {
		return list, err
	}

	if !redactionRequired(ctx, kind.Type()) {
		return list, nil
	}

	for idx, res := range list.Items {
		list.Items[idx] = redactResource(res)
	}

	return list, nil
}

// Create implements state.CoreState interface.
func (r *redactingState) Create(ctx context.Context, res resource.Resource, opts ...state.CreateOption) error {
	return r.state.Create(ctx, res, opts...)
}

// Update implements state.CoreState interface.
func (r *redactingState) Update(ctx context.Context, newResource resource.Resource, opts ...state.UpdateOption) error {
	return r.state.Update(ctx, newResource, opts...)
}

// Destroy implements state.CoreState interface.
func (r *redactingState) Destroy(ctx context.Context, ptr resource.Pointer, opts ...state.DestroyOption) error {
	return r.state.Destroy(ctx, ptr, opts...)
}

// Watch implements state.CoreState interface.
func (r *redactingState) Watch(ctx context.Context, ptr resource.Pointer, ch chan<- state.Event, opts ...state.WatchOption) error {
	if !redactionRequired(ctx, ptr.Type()) {
		return r.state.Watch(ctx, ptr, ch, opts...)
	}

	inCh := make(chan state.Event)

	if err := r.state.Watch(ctx, ptr, inCh, opts...); err != nil {
		return err
	}

	go pump(ctx, inCh, ch, redactEvent)

	return nil
}

// WatchKind implements state.CoreState interface.
func (r *redactingState) WatchKind(ctx context.Context, kind resource.Kind, ch chan<- state.Event, opts ...state.WatchKindOption) error {
	if !redactionRequired(ctx, kind.Type()) {
		return r.state.WatchKind(ctx, kind, ch, opts...)
	}

	inCh := make(chan state.Event)

	if err := r.state.WatchKind(ctx, kind, inCh, opts...); err != nil {
		return err
	}

	go pump(ctx, inCh, ch, redactEvent)

	return nil
}

// WatchKindAggregated implements state.CoreState interface.
func (r *redactingState) WatchKindAggregated(ctx context.Context, kind resource.Kind, ch chan<- []state.Event, opts ...state.WatchKindOption) error {
	if !redactionRequired(ctx, kind.Type()) {
		return r.state.WatchKindAggregated(ctx, kind, ch, opts...)
	}

	inCh := make(chan []state.Event)

	if err := r.state.WatchKindAggregated(ctx, kind, inCh, opts...); err != nil {
		return err
	}

	go pump(ctx, inCh, ch, redactEvents)

	return nil
}

// pump forwards the events from in to out, passing them through transform.
//
// It terminates when the context is canceled, which is also what terminates the watch feeding in.
func pump[T any](ctx context.Context, in <-chan T, out chan<- T, transform func(T) T) {
	for {
		var item T

		select {
		case <-ctx.Done():
			return
		case item = <-in:
		}

		select {
		case <-ctx.Done():
			return
		case out <- transform(item):
		}
	}
}

func redactEvent(event state.Event) state.Event {
	event.Resource = redactResource(event.Resource)
	event.Old = redactResource(event.Old)

	return event
}

func redactEvents(events []state.Event) []state.Event {
	// the slice might be shared with the other watchers, so build a new one
	result := make([]state.Event, 0, len(events))

	for _, event := range events {
		result = append(result, redactEvent(event))
	}

	return result
}

// redactResource returns the resource with its sensitive values redacted.
//
// The resource is copied before being redacted, as it might share the backing storage with the one
// kept in the state. Resources which carry nothing to redact, tombstones included, are returned as
// they are, so the annotation marks exactly the resources which lost something on the way out.
func redactResource(res resource.Resource) resource.Resource { //nolint:ireturn
	if res == nil {
		return nil
	}

	// a destroy event carries a tombstone, which stands for a resource which is gone: it holds the
	// metadata alone, and asking it for a spec panics. There is nothing to redact in it, and nothing
	// which would need redacting -- whether this type may be watched at all was already decided by
	// [AccessPolicy], which doesn't look at the resource.
	if resource.IsTombstone(res) {
		return res
	}

	spec, ok := res.Spec().(redactable.Spec)
	if !ok || !spec.HasSecrets(*res.Metadata()) {
		return res
	}

	res = res.DeepCopy()

	//nolint:forcetypeassert,errcheck
	res.Spec().(redactable.Spec).RedactSecrets(*res.Metadata())

	res.Metadata().Annotations().Set(constants.AnnotationRedacted, constants.AnnotationRedactedValue)

	return res
}

// redactionRequired reports whether the requester should get the resources of the type in the redacted form.
func redactionRequired(ctx context.Context, resourceType resource.Type) bool {
	if !redactable.IsRedactable(resourceType) {
		// either a non-sensitive type, which is served in full to anyone, or a sensitive one which
		// [AccessPolicy] serves to an os:admin alone, and in full
		return false
	}

	mode, err := redactable.ModeFromIncomingContext(ctx)
	if err != nil {
		// AccessPolicy rejects a request carrying an unparseable header before it ever reaches the
		// state, so this is unreachable; redact anyway rather than serve a value in full by accident
		return true
	}

	switch mode {
	case redactable.ModeEnforce:
		return true
	case redactable.ModeAccept:
		// an admin is entitled to see the resource in full, so there is nothing to hide from it
		return !authz.GetRoles(ctx).Includes(role.Admin)
	case redactable.ModeDisabled:
		return false
	default:
		return true
	}
}
