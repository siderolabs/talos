// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package internal_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	storaged "github.com/siderolabs/talos/internal/app/storaged"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestGetSystemDiskPathsForListing(t *testing.T) {
	for _, test := range []struct {
		name        string
		secondaries []string
		createVDA   bool
		expected    []string
	}{
		{
			name:        "complete topology",
			secondaries: []string{"vda"},
			createVDA:   true,
			expected: []string{
				"/dev/disk/by-id/md-name-talos:boot",
				"/dev/md127",
				"/dev/vda",
			},
		},
		{
			name:        "incomplete topology falls back to direct system disk",
			secondaries: []string{"vda"},
			expected: []string{
				"/dev/disk/by-id/md-name-talos:boot",
				"/dev/md127",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			st := state.WrapCore(namespaced.NewState(inmem.Build))

			systemDisk := block.NewSystemDisk(block.NamespaceName, block.SystemDiskID)
			systemDisk.TypedSpec().DiskID = "md127"
			systemDisk.TypedSpec().DevPath = "/dev/disk/by-id/md-name-talos:boot"
			require.NoError(t, st.Create(ctx, systemDisk))

			md := block.NewDevice(block.NamespaceName, "md127")
			md.TypedSpec().Secondaries = test.secondaries
			require.NoError(t, st.Create(ctx, md))

			if test.createVDA {
				require.NoError(t, st.Create(ctx, block.NewDevice(block.NamespaceName, "vda")))
			}

			paths, err := storaged.GetSystemDiskPathsForListing(ctx, st)
			require.NoError(t, err)
			assert.ElementsMatch(t, test.expected, paths)
		})
	}
}
