/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package packet

import (
	"net"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config"
)

const (
	// PacketUserDataEndpoint is the local metadata endpoint for Packet.
	PacketUserDataEndpoint = "https://metadata.packet.net/userdata"
)

// Packet is a discoverer for non-cloud environments.
type Packet struct{}

// Name implements the platform.Platform interface.
func (p *Packet) Name() string {
	return "Packet"
}

// Configuration implements the platform.Platform interface.
func (p *Packet) Configuration() ([]byte, error) {
	return config.Download(PacketUserDataEndpoint)
}

// Mode implements the platform.Platform interface.
func (p *Packet) Mode() runtime.Mode {
	return runtime.Metal
}

// Hostname implements the platform.Platform interface.
func (p *Packet) Hostname() (hostname []byte, err error) {
	return nil, nil
}

// ExternalIPs provides any external addresses assigned to the instance
func (p *Packet) ExternalIPs() (addrs []net.IP, err error) {
	return addrs, err
}
