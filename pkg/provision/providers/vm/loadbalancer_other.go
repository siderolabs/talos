// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !darwin && !linux

package vm

import "net/netip"

func getLbBindIP(_ netip.Addr) string {
	return "0.0.0.0"
}
