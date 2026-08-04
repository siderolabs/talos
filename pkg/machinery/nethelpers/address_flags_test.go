// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

func TestAddressFlagsManaged(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		flags nethelpers.AddressFlags

		expectedManaged   nethelpers.AddressFlags
		expectedUnmanaged nethelpers.AddressFlags
	}{
		{
			name:  "empty",
			flags: 0,
		},
		{
			name:  "permanent",
			flags: nethelpers.AddressFlags(nethelpers.AddressPermanent),

			expectedManaged: nethelpers.AddressFlags(nethelpers.AddressPermanent),
		},
		{
			name:  "secondary IPv4 address",
			flags: nethelpers.AddressFlags(nethelpers.AddressPermanent | nethelpers.AddressTemporary),

			expectedManaged:   nethelpers.AddressFlags(nethelpers.AddressPermanent),
			expectedUnmanaged: nethelpers.AddressFlags(nethelpers.AddressTemporary),
		},
		{
			name:  "IPv6 address in DAD",
			flags: nethelpers.AddressFlags(nethelpers.AddressPermanent | nethelpers.AddressTentative | nethelpers.AddressOptimistic),

			expectedManaged:   nethelpers.AddressFlags(nethelpers.AddressPermanent),
			expectedUnmanaged: nethelpers.AddressFlags(nethelpers.AddressTentative | nethelpers.AddressOptimistic),
		},
		{
			name:  "SLAAC address",
			flags: nethelpers.AddressFlags(nethelpers.AddressManagementTemp | nethelpers.AddressStablePrivacy | nethelpers.AddressDeprecated),

			expectedManaged:   nethelpers.AddressFlags(nethelpers.AddressManagementTemp),
			expectedUnmanaged: nethelpers.AddressFlags(nethelpers.AddressStablePrivacy | nethelpers.AddressDeprecated),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.expectedManaged, test.flags.Managed())
			assert.Equal(t, test.expectedUnmanaged, test.flags.Unmanaged())
			assert.Equal(t, test.flags, test.flags.Managed()|test.flags.Unmanaged())
		})
	}
}
