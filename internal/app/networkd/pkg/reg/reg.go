/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"net"

	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/app/networkd/proto"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
)

// Registrator is the concrete type that implements the factory.Registrator and
// proto.Init interfaces.
type Registrator struct {
	Networkd *networkd.Networkd
}

// NewRegistrator builds new Registrator instance.
func NewRegistrator(n *networkd.Networkd) *Registrator {
	return &Registrator{
		Networkd: n,
	}
}

// Register implements the factory.Registrator interface.
func (r *Registrator) Register(s *grpc.Server) {
	proto.RegisterNetworkdServer(s, r)
}

func toCIDR(family uint8, prefix net.IP, prefixLen int) string {
	var netLen = 32
	if family == unix.AF_INET6 {
		netLen = 128
	}
	ipNet := &net.IPNet{
		IP:   prefix,
		Mask: net.CIDRMask(prefixLen, netLen),
	}
	return ipNet.String()
}
