// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/container/internal/files"
)

func TestReadResolvConf(t *testing.T) {
	t.Parallel()

	spec, err := files.ReadResolvConf("testdata/resolv.conf")
	require.NoError(t, err)

	require.Equal(t, []netip.Addr{
		netip.MustParseAddr("127.0.0.53"),
		netip.MustParseAddr("::1"),
	}, spec.DNSServers)
}
