// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm_test

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

func TestVRFPeerAddress(t *testing.T) {
	t.Parallel()

	assert.Equal(t, netip.MustParsePrefix("192.0.2.2/30"), vm.VRFPeerPrefix())
	assert.Equal(t, netip.MustParseAddr("192.0.2.2"), vm.VRFPeerAddress())
}
