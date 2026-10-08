// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remote_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/remote"
)

func TestNUMAOptionsRoundTrip(t *testing.T) {
	for _, count := range []int{0, 1, 3} {
		data, err := remote.MarshalOptions([]provision.Option{provision.WithNUMANodes(count)})
		require.NoError(t, err)

		var wire map[string]any
		require.NoError(t, json.Unmarshal(data, &wire))

		if count == 0 {
			require.NotContains(t, wire, "numa_nodes")
		} else {
			require.Equal(t, float64(count), wire["numa_nodes"])
		}

		opts, err := remote.UnmarshalOptions(data)
		require.NoError(t, err)

		result := provision.DefaultOptions()
		for _, opt := range opts {
			require.NoError(t, opt(&result))
		}

		require.Equal(t, count, result.NUMANodes)
	}

	for _, data := range [][]byte{nil, []byte(`{}`)} {
		opts, err := remote.UnmarshalOptions(data)
		require.NoError(t, err)

		result := provision.DefaultOptions()
		for _, opt := range opts {
			require.NoError(t, opt(&result))
		}

		require.Zero(t, result.NUMANodes)
	}

	_, err := remote.MarshalOptions([]provision.Option{provision.WithNUMANodes(-1)})
	require.Error(t, err)
}
