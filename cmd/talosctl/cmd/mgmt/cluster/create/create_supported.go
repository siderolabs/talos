// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux || darwin

package create

import (
	"net/netip"

	"github.com/siderolabs/siderolink/pkg/wireguard"
)

func generateRandomNodeAddr(prefix netip.Prefix) (netip.Prefix, error) {
	return wireguard.GenerateRandomNodeAddr(prefix)
}

func networkPrefix(prefix string) (netip.Prefix, error) {
	return wireguard.NetworkPrefix(prefix), nil
}
