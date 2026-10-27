// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mutationevidence_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage/internal/mutationevidence"
	libvirtstorage "github.com/siderolabs/talos/internal/pkg/libvirt/storage"
)

func TestMerge(t *testing.T) {
	t.Parallel()

	outcomes := []libvirtstorage.OperationOutcome{
		libvirtstorage.NoMutationSubmitted,
		libvirtstorage.Finished,
		libvirtstorage.Unknown,
		255,
	}
	expected := [][]libvirtstorage.OperationOutcome{
		{libvirtstorage.NoMutationSubmitted, libvirtstorage.Finished, libvirtstorage.Unknown, libvirtstorage.Unknown},
		{libvirtstorage.Finished, libvirtstorage.Finished, libvirtstorage.Unknown, libvirtstorage.Unknown},
		{libvirtstorage.Unknown, libvirtstorage.Unknown, libvirtstorage.Unknown, libvirtstorage.Unknown},
		{libvirtstorage.Unknown, libvirtstorage.Unknown, libvirtstorage.Unknown, libvirtstorage.Unknown},
	}

	for i, previous := range outcomes {
		for j, next := range outcomes {
			t.Run(fmt.Sprintf("%d/%d", previous, next), func(t *testing.T) {
				t.Parallel()
				assert.Equal(t, expected[i][j], mutationevidence.Merge(previous, next))
			})
		}
	}
}

func TestCanRelease(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name              string
		outcome           libvirtstorage.OperationOutcome
		beforePublication bool
		afterPublication  bool
	}{
		{"no mutation", libvirtstorage.NoMutationSubmitted, true, true},
		{"finished", libvirtstorage.Finished, false, true},
		{"unknown", libvirtstorage.Unknown, false, false},
		{"invalid", 255, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.beforePublication, mutationevidence.CanRelease(tt.outcome, false))
			assert.Equal(t, tt.afterPublication, mutationevidence.CanRelease(tt.outcome, true))
		})
	}
}
