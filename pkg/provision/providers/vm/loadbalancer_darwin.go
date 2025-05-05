// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import "net/netip"

// getLbBindIP returns the 0.0.0.0 address to bind to all interfaces on macos.
// The bridge interface address is not used as the bridge is not yet created at this stage.
// Multiple loadbalancers can be assigned via different ports.
func getLbBindIP(_ netip.Addr) string {
	return "0.0.0.0"
}
