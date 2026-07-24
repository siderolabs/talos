// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"net/netip"
	"os/exec"
)

func configureBGPVRFPeerRoute(bridge string, peer, source netip.Addr) error {
	output, err := exec.Command( //nolint:noctx
		"ip",
		"route",
		"replace",
		peer.String()+"/32",
		"dev",
		bridge,
		"src",
		source.String(),
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error configuring BGP VRF peer route: %w: %s", err, output)
	}

	return nil
}
