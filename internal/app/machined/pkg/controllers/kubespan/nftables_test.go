// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package kubespan_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

func TestNfTables(t *testing.T) {
	// use a different mark to avoid conflicts with running kubespan
	mgr := kubespan.NewNfTablesManager(constants.KubeSpanDefaultFirewallMark+10, constants.KubeSpanDefaultForceFirewallMark+10)

	// cleanup should be fine if nothing is installed
	assert.NoError(t, mgr.Cleanup())

	defer mgr.Cleanup() //nolint:errcheck

	var builder netaddr.IPSetBuilder

	builder.AddPrefix(netaddr.MustParseIPPrefix("172.20.0.0/24"))
	builder.AddPrefix(netaddr.MustParseIPPrefix("10.0.0.0/16"))

	ipSet, err := builder.IPSet()
	require.NoError(t, err)

	assert.NoError(t, mgr.Update(ipSet))

	builder.AddPrefix(netaddr.MustParseIPPrefix("10.0.0.0/8"))

	ipSet, err = builder.IPSet()
	require.NoError(t, err)

	assert.NoError(t, mgr.Update(ipSet))

	assert.NoError(t, mgr.Cleanup())
}
