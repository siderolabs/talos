// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Additional information can be found
// https://www.kernel.org/doc/Documentation/networking/bonding.txt.

package nic

import (
	"fmt"
	"net"

	"github.com/talos-systems/talos/pkg/machinery/config"
)

// WithVIPConfig adapts a talosconfig VIP configuration to a networkd interface configuration option.
func WithVIPConfig(cfg config.VIPConfig) Option {
	return func(n *NetworkInterface) (err error) {
		sharedIP := net.ParseIP(cfg.IP())
		if sharedIP == nil {
			return fmt.Errorf("failed to parse shared IP %q as an IP address", cfg.IP())
		}

		n.VirtualIP = sharedIP

		return nil
	}
}
