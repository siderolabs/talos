// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import "net/netip"

// getLbBindIP returns the gateway address to bind the loadbalancer to the bridge interface.
func getLbBindIP(gateway netip.Addr) string {
	return gateway.String()
}
