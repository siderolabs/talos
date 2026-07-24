// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/provision"
)

func TestHTTPProbeRequestNormalize(t *testing.T) {
	request := provision.HTTPProbeRequest{
		IP:   netip.MustParseAddr("203.0.113.100"),
		Port: 80,
		Path: "/",
	}

	normalized, err := request.Normalize()
	require.NoError(t, err)
	require.Equal(t, provision.HTTPProbeDefaultTimeout, normalized.Timeout)

	request.Timeout = time.Millisecond
	normalized, err = request.Normalize()
	require.NoError(t, err)
	require.Equal(t, provision.HTTPProbeMinTimeout, normalized.Timeout)

	request.Timeout = time.Minute
	normalized, err = request.Normalize()
	require.NoError(t, err)
	require.Equal(t, provision.HTTPProbeMaxTimeout, normalized.Timeout)
}
