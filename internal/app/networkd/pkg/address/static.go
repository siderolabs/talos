// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package address

import (
	"context"
	"log"
	"net"
	"time"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/machinery/config"
)

const staticRouteDefaultMetric uint32 = 10

// Static implements the Addressing interface.
type Static struct {
	CIDR        string
	Mtu         int
	FQDN        string
	RouteList   []config.Route
	NetIf       *net.Interface
	NameServers []net.IP
}

// Discover doesnt do anything in the static configuration since all
// the necessary configuration data is supplied via config.
func (s *Static) Discover(ctx context.Context, logger *log.Logger, link *net.Interface) error {
	s.NetIf = link

	return nil
}

// Name returns back the name of the address method.
func (s *Static) Name() string {
	return "static"
}

// Address returns the IP address.
func (s *Static) Address() *net.IPNet {
	var ip net.IP

	var ipn *net.IPNet

	if s.CIDR != "" {
		//nolint:errcheck
		ip, ipn, _ = net.ParseCIDR(s.CIDR)
		ipn.IP = ip
	}

	return ipn
}

// Mask returns the netmask.
func (s *Static) Mask() net.IPMask {
	//nolint:errcheck
	_, ipnet, _ := net.ParseCIDR(s.CIDR)

	return ipnet.Mask
}

// MTU returns the specified MTU.
func (s *Static) MTU() uint32 {
	mtu := uint32(s.Mtu)
	if mtu == 0 {
		mtu = uint32(s.NetIf.MTU)
	}

	return mtu
}

// TTL returns the address lifetime. Since this is static, there is
// no TTL (0).
func (s *Static) TTL() time.Duration {
	return 0
}

// Family qualifies the address as ipv4 or ipv6.
func (s *Static) Family() int {
	if s.Address() == nil {
		panic("unable to determine address family as address is nil")
	}

	if s.Address().IP.To4() != nil {
		return unix.AF_INET
	}

	return unix.AF_INET6
}

// Scope sets the address scope.
func (s *Static) Scope() uint8 {
	return unix.RT_SCOPE_UNIVERSE
}

// Routes aggregates the specified routes for a given device configuration
// TODO: do we need to be explicit on route vs gateway?
func (s *Static) Routes() (routes []*Route) {
	for _, route := range s.RouteList {
		_, ipnet, err := net.ParseCIDR(route.Network())
		if err != nil {
			// TODO: we should at least log the error
			continue
		}

		metric := staticRouteDefaultMetric

		if route.Metric() != 0 {
			metric = route.Metric()
		}

		routes = append(routes, &Route{
			Destination: ipnet,
			Gateway:     net.ParseIP(route.Gateway()),
			Metric:      metric,
		})
	}

	return routes
}

// Resolvers returns the DNS resolvers.
func (s *Static) Resolvers() []net.IP {
	return s.NameServers
}

// Hostname returns the hostname.
func (s *Static) Hostname() string {
	return s.FQDN
}

// Link returns the underlying net.Interface that this address
// method is configured for.
func (s Static) Link() *net.Interface {
	return s.NetIf
}

// Valid denotes if this address method should be used.
func (s *Static) Valid() bool {
	return true
}
