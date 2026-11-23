// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package volumes ties API streams to the lifetime of the volumes they read from.
package volumes

import (
	"context"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// UntilTeardown returns a context which ends with ctx, or once the volume lifecycle stops running.
//
// The API serves until after volumes are finalized, so a follow holding a file open on a volume
// would keep that volume busy and block its unmount.
func UntilTeardown(ctx context.Context, st state.State) (context.Context, error) {
	ctx, cancel := context.WithCancel(ctx)

	events := make(chan state.Event)

	if err := st.Watch(ctx, block.NewVolumeLifecycle(block.NamespaceName, block.VolumeLifecycleID).Metadata(), events); err != nil {
		cancel()

		return nil, err
	}

	go func() {
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return
			case event := <-events:
				if event.Type == state.Errored || event.Type == state.Destroyed || event.Resource.Metadata().Phase() != resource.PhaseRunning {
					return
				}
			}
		}
	}()

	return ctx, nil
}
