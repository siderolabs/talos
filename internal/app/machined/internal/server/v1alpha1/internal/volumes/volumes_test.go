// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/internal/server/v1alpha1/internal/volumes"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestUntilTeardown(t *testing.T) {
	lifecycle := block.NewVolumeLifecycle(block.NamespaceName, block.VolumeLifecycleID).Metadata()

	for _, tc := range []struct {
		name   string
		create bool
		end    func(context.Context, state.State) error
	}{
		{
			name:   "teardown",
			create: true,
			end: func(ctx context.Context, st state.State) error {
				_, err := st.Teardown(ctx, lifecycle)

				return err
			},
		},
		{
			name:   "destroy",
			create: true,
			end:    func(ctx context.Context, st state.State) error { return st.Destroy(ctx, lifecycle) },
		},
		{
			name: "missing lifecycle",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			st := state.WrapCore(inmem.NewState())

			if tc.create {
				require.NoError(t, st.Create(ctx, block.NewVolumeLifecycle(block.NamespaceName, block.VolumeLifecycleID)))
			}

			streamCtx, err := volumes.UntilTeardown(ctx, st)
			require.NoError(t, err)

			if tc.end != nil {
				select {
				case <-streamCtx.Done():
					t.Fatal("stream ended while volumes are running")
				case <-time.After(100 * time.Millisecond):
				}

				require.NoError(t, tc.end(ctx, st))
			}

			select {
			case <-streamCtx.Done():
			case <-time.After(5 * time.Second):
				t.Fatal("stream outlived the volume lifecycle")
			}

			require.NoError(t, ctx.Err())
		})
	}
}
