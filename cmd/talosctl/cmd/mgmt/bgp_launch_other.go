// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !linux

package mgmt

import (
	"context"
	"errors"

	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
)

// fabricZebra programs the host kernel FIB (the "zebra" role) and so requires Linux netlink. The
// cross-platform unnumbered peer (gobgp + Router Advertisements) runs without it.
func fabricZebra(context.Context, *gobgpsrv.BgpServer, []string, string) error {
	return errors.New("--bgp-zebra (host FIB programming) is only supported on Linux")
}
