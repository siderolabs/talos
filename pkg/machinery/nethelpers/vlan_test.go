// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

func TestVLANLinkName(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		base   string
		vlanID uint16

		expected string
	}{
		{
			base:   "eth0",
			vlanID: 1,

			expected: "eth0.1",
		},
		{
			base:   "en9s0",
			vlanID: 4095,

			expected: "en9s0.4095",
		},
		{
			base:   "0123456789",
			vlanID: 4095,

			expected: "0123456789.4095",
		},
		{
			base:   "enx12545f8c99cd",
			vlanID: 25,

			expected: "enx1ee6413.25",
		},
		{
			base:   "enx12545f8c99cd",
			vlanID: 4095,

			expected: "enx1ee6413.4095",
		},
		{
			base:   "enx12545f8c99ce",
			vlanID: 4095,

			expected: "enx1ef972f.4095",
		},
	} {
		t.Run(fmt.Sprintf("%s.%d", test.base, test.vlanID), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expected, nethelpers.VLANLinkName(test.base, test.vlanID))
		})
	}
}
