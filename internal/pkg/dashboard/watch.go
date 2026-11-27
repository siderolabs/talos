// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dashboard

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
)

// watchKinds watches all the given kinds of a namespace, with bootstrap contents, and passes every
// event to onEvent until ctx is canceled.
//
// One channel keeps the kinds in step. onEvent runs on the watch goroutine, so callers hand off to
// the UI goroutine. onSetupError is called once if a watch cannot be set up.
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
