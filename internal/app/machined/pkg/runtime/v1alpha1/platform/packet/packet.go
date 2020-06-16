// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package packet

import (
	"log"
	"net"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/download"
)

const (
	// PacketUserDataEndpoint is the local metadata endpoint for Packet.
	PacketUserDataEndpoint = "https://metadata.packet.net/userdata"
)

// Packet is a discoverer for non-cloud environments.
type Packet struct{}

// Name implements the platform.Platform interface.
func (p *Packet) Name() string {
	return "packet"
}

// Configuration implements the platform.Platform interface.
func (p *Packet) Configuration() ([]byte, error) {
	log.Printf("fetching machine config from: %q", PacketUserDataEndpoint)

	return download.Download(PacketUserDataEndpoint)
}

// Mode implements the platform.Platform interface.
func (p *Packet) Mode() runtime.Mode {
	return runtime.ModeMetal
}

// Hostname implements the platform.Platform interface.
func (p *Packet) Hostname() (hostname []byte, err error) {
	return nil, nil
}

// ExternalIPs implements the runtime.Platform interface.
func (p *Packet) ExternalIPs() (addrs []net.IP, err error) {
	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (p *Packet) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS1,115200n8"),
	}
}
