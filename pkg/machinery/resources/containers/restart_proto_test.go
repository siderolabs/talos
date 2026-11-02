// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"

	"github.com/siderolabs/protoenc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	containerspb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/containers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// TestRestartMaxRetriesPresence covers maxRetries across the API: unset (unlimited) and 0 (never
// retry) stay apart in both directions.
func TestRestartMaxRetriesPresence(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		maxRetries *uint32
	}{
		{name: "unlimited"},
		{name: "zero", maxRetries: new(uint32(0))},
		{name: "three", maxRetries: new(uint32(3))},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			original := containers.ContainerRestartSpec{
				Policy:     containers.ContainerRestartPolicyOnFailure,
				MaxRetries: test.maxRetries,
			}

			wire, err := protoenc.Marshal(&original)
			require.NoError(t, err)

			var unmarshaled containerspb.ContainerRestartSpec

			require.NoError(t, proto.Unmarshal(wire, &unmarshaled))
			assert.Equal(t, test.maxRetries, unmarshaled.MaxRetries)

			wire, err = proto.Marshal(&unmarshaled)
			require.NoError(t, err)

			var roundTripped containers.ContainerRestartSpec

			require.NoError(t, protoenc.Unmarshal(wire, &roundTripped))
			assert.Equal(t, original, roundTripped)
		})
	}
}
