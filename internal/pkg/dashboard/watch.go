// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// watchKinds watches all the given resource kinds of a namespace, with their bootstrap contents,
// and passes every event to onEvent until ctx is canceled.
//
// A single event channel keeps the kinds in step: screens join the kinds, so rendering on every
// event from any of them is both correct and simpler than reconciling separate streams. onEvent
// is called from the watch goroutine, and callers hand the work over to the UI goroutine.
//
// If a watch cannot be set up, onSetupError is called once and the function returns.
func watchKinds(
	ctx context.Context,
	st state.State,
	namespace resource.Namespace,
	kinds []resource.Type,
	onEvent func(state.Event),
	onSetupError func(error),
) {
	eventCh := make(chan state.Event)

	for _, kind := range kinds {
		md := resource.NewMetadata(namespace, kind, "", resource.VersionUndefined)

		if err := st.WatchKind(ctx, &md, eventCh, state.WithBootstrapContents(true)); err != nil {
			onSetupError(err)

			return
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-eventCh:
			onEvent(event)
		}
	}
}

// storeOrDelete files a watched resource into the map of its kind, or removes it when destroyed.
func storeOrDelete[T any](target map[string]T, id string, value T, destroyed bool) {
	if destroyed {
		delete(target, id)

		return
	}

	target[id] = value
}
