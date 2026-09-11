// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remote_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/remote"
)

func TestLLDPOptionsRoundTrip(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		data, err := remote.MarshalOptions([]provision.Option{provision.WithLLDP(enabled)})
		require.NoError(t, err)

		opts, err := remote.UnmarshalOptions(data)
		require.NoError(t, err)

		result := provision.DefaultOptions()
		for _, opt := range opts {
			require.NoError(t, opt(&result))
		}

		require.Equal(t, enabled, result.LLDPEnabled)
	}

	opts, err := remote.UnmarshalOptions([]byte(`{}`))
	require.NoError(t, err)

	result := provision.DefaultOptions()
	for _, opt := range opts {
		require.NoError(t, opt(&result))
	}

	require.False(t, result.LLDPEnabled)
}
