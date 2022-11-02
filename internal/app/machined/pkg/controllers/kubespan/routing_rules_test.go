// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package kubespan_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestRoutingRules(t *testing.T) {
	// use a different table/mark to avoid conflicts with running kubespan
	mgr := kubespan.NewRulesManager(constants.KubeSpanDefaultRoutingTable+10, constants.KubeSpanDefaultForceFirewallMark<<1, constants.KubeSpanDefaultFirewallMask<<1)

	// cleanup should be fine if nothing is installed
	assert.NoError(t, mgr.Cleanup())

	defer mgr.Cleanup() //nolint:errcheck

	assert.NoError(t, mgr.Install())
	assert.NoError(t, mgr.Cleanup())
}
