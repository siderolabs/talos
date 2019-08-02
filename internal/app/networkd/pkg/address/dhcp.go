/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package address

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/constants"
	"golang.org/x/sys/unix"
)

// DHCP implements the Addressing interface
type DHCP struct {
	Ack *dhcpv4.DHCPv4
}

// Name returns back the name of the address method.
func (d *DHCP) Name() string {
	return "dhcp"
}

// Discover handles the DHCP client exchange stores the DHCP Ack.
func (d *DHCP) Discover(ctx context.Context, name string) error {
	// TODO do something with context
	ack, err := discover(name)
	d.Ack = ack
	return err
}

// Address returns back the IP address from the received DHCP offer.
func (d *DHCP) Address() net.IP {
	return d.Ack.YourIPAddr
}

// Mask returns the netmask from the DHCP offer.
func (d *DHCP) Mask() net.IPMask {
	return d.Ack.SubnetMask()
}

// MTU returs the MTU size from the DHCP offer.
func (d *DHCP) MTU() uint32 {
	// TODO do we need to implement dhcpv4.GetUint32 upstream?
	mtu, err := dhcpv4.GetUint16(dhcpv4.OptionInterfaceMTU, d.Ack.Options)
	if err != nil {
		return 1500
	}
	return uint32(mtu)
}

// TTL denotes how long a DHCP offer is valid for.
func (d *DHCP) TTL() time.Duration {
	if d.Ack == nil {
		return 0
	}
	return d.Ack.IPAddressLeaseTime(time.Minute * 30)
}

// Family qualifies the address as ipv4 or ipv6
func (d *DHCP) Family() uint8 {
	if d.Ack.YourIPAddr.To4() != nil {
		return unix.AF_INET
	}
	return unix.AF_INET6
}

// Scope sets the address scope
func (d *DHCP) Scope() uint8 {
	return unix.RT_SCOPE_UNIVERSE
}

// Routes aggregates all Routers and ClasslessStaticRoutes retrieved from
// the DHCP offer.
func (d *DHCP) Routes() (routes []*Route) {
	for _, router := range d.Ack.Router() {
		// Note, we don't set a Dest on routes generated from Router()
		// since these all should be gateways ( listed in order of preference )
		routes = append(routes, &Route{Router: router})
	}
	routes = append(routes, d.Ack.ClasslessStaticRoute()...)
	return routes
}

// Resolvers returns the DNS resolvers from the DHCP offer.
func (d *DHCP) Resolvers() []net.IP {
	return d.Ack.DNS()
}

// Hostname returns the hostname from the DHCP offer.
func (d *DHCP) Hostname() string {
	// Truncate the returned hostname to only return
	// the actual host entry
	return strings.Split(d.Ack.HostName(), ".")[0]
}

// discover handles the actual DHCP conversation.
func discover(name string) (*dhcpv4.DHCPv4, error) {
	opts := []dhcpv4.OptionCode{
		dhcpv4.OptionClasslessStaticRoute,
		dhcpv4.OptionDomainNameServer,
		dhcpv4.OptionDNSDomainSearchList,
		dhcpv4.OptionHostName,
		// TODO: handle these options
		dhcpv4.OptionNTPServers,
		dhcpv4.OptionDomainName,
	}

	// <3 azure
	// When including dhcp.OptionInterfaceMTU we don't get a dhcp offer back on azure.
	// So we'll need to explicitly exclude adding this option for azure.
	if p := kernel.ProcCmdline().Get(constants.KernelParamPlatform).First(); p != nil {
		if *p != "azure" {
			opts = append(opts, dhcpv4.OptionInterfaceMTU)
		}
	}

	mods := []dhcpv4.Modifier{dhcpv4.WithRequestedOptions(opts...)}

	// TODO expose this with some debug logging option
	cli, err := nclient4.New(name, nclient4.WithTimeout(10*time.Second), nclient4.WithDebugLogger())
	//cli, err := nclient4.New(name, nclient4.WithTimeout(2*time.Second))
	if err != nil {
		log.Println("failed nclient4.new")
		return nil, err
	}
	// nolint: errcheck
	defer cli.Close()

	_, ack, err := cli.Request(context.Background(), mods...)
	if err != nil {
		// TODO: Make this a well defined error so we can make it not fatal
		log.Println("failed dhcp request")
		return nil, err
	}

	return ack, err
}
