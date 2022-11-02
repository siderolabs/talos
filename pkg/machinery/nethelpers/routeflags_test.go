// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

func TestRouteFlagsStrings(t *testing.T) {
	for _, tt := range []struct {
		routeFlagsString string
		expected         nethelpers.RouteFlags
	}{
		{
			routeFlagsString: "",
			expected:         nethelpers.RouteFlags(0),
		},
		{
			routeFlagsString: "cloned",
			expected:         nethelpers.RouteFlags(nethelpers.RouteCloned),
		},
		{
			routeFlagsString: "cloned,prefix",
			expected:         nethelpers.RouteFlags(nethelpers.RouteCloned | nethelpers.RoutePrefix),
		},
	} {
		routeFlags, err := nethelpers.RouteFlagsString(tt.routeFlagsString)

		assert.NoError(t, err)
		assert.Equal(t, tt.expected, routeFlags)
	}
}
