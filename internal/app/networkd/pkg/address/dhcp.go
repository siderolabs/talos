// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package address

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// DHCP implements the Addressing interface.
type DHCP struct {
	Ack   *dhcpv4.DHCPv4
	NetIf *net.Interface
}

// Name returns back the name of the address method.
func (d *DHCP) Name() string {
	return "dhcp"
}

// Link returns the underlying net.Interface that this address
// method is configured for.
func (d *DHCP) Link() *net.Interface {
	return d.NetIf
}

// Discover handles the DHCP client exchange stores the DHCP Ack.
func (d *DHCP) Discover(ctx context.Context, link *net.Interface) error {
	d.NetIf = link
	// TODO do something with context
	ack, err := d.discover()
	d.Ack = ack

	return err
}

// Address returns back the IP address from the received DHCP offer.
func (d *DHCP) Address() *net.IPNet {
	return &net.IPNet{
		IP:   d.Ack.YourIPAddr,
		Mask: d.Mask(),
	}
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
		return uint32(d.NetIf.MTU)
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

// Family qualifies the address as ipv4 or ipv6.
func (d *DHCP) Family() int {
	if d.Ack.YourIPAddr.To4() != nil {
		return unix.AF_INET
	}

	return unix.AF_INET6
}

// Scope sets the address scope.
func (d *DHCP) Scope() uint8 {
	return unix.RT_SCOPE_UNIVERSE
}

// Valid denotes if this address method should be used.
func (d *DHCP) Valid() bool {
	return d.Ack != nil
}

// Routes aggregates all Routers and ClasslessStaticRoutes retrieved from
// the DHCP offer.
// rfc3442:
//   If the DHCP server returns both a Classless Static Routes option and
//   a Router option, the DHCP client MUST ignore the Router option.
func (d *DHCP) Routes() (routes []*Route) {
	if len(d.Ack.ClasslessStaticRoute()) > 0 {
		return d.Ack.ClasslessStaticRoute()
	}

	defRoute := &net.IPNet{
		IP:   net.IPv4zero,
		Mask: net.IPv4Mask(0, 0, 0, 0),
	}

	for _, router := range d.Ack.Router() {
		routes = append(routes, &Route{Router: router, Dest: defRoute})
	}

	return routes
}

// Resolvers returns the DNS resolvers from the DHCP offer.
func (d *DHCP) Resolvers() []net.IP {
	return d.Ack.DNS()
}

// Hostname returns the hostname from the DHCP offer.
func (d *DHCP) Hostname() (hostname string) {
	if d.Ack.HostName() == "" {
		hostname = fmt.Sprintf("%s-%s", "talos", strings.ReplaceAll(d.Address().IP.String(), ".", "-"))
	} else {
		hostname = d.Ack.HostName()
	}

	if d.Ack.DomainName() != "" {
		hostname = fmt.Sprintf("%s.%s", strings.Split(hostname, ".")[0], d.Ack.DomainName())
	}

	return hostname
}

// discover handles the actual DHCP conversation.
func (d *DHCP) discover() (*dhcpv4.DHCPv4, error) {
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
	if p := procfs.ProcCmdline().Get(constants.KernelParamPlatform).First(); p != nil {
		if *p != "azure" {
			opts = append(opts, dhcpv4.OptionInterfaceMTU)
		}
	}

	mods := []dhcpv4.Modifier{dhcpv4.WithRequestedOptions(opts...)}

	// TODO expose this ( nclient4.WithDebugLogger() ) with some
	// debug logging option
	cli, err := nclient4.New(d.NetIf.Name)
	if err != nil {
		return nil, err
	}

	// nolint: errcheck
	defer cli.Close()

	_, ack, err := cli.Request(context.Background(), mods...)
	if err != nil {
		// TODO: Make this a well defined error so we can make it not fatal
		log.Println("failed dhcp request for", d.NetIf.Name)
		return nil, err
	}

	return ack, err
}
