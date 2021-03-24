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
	"github.com/talos-systems/go-procfs/procfs"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

const dhcpReceivedRouteMetric uint32 = 1024

// DHCP4 implements the Addressing interface.
type DHCP4 struct {
	Offer       *dhcpv4.DHCPv4
	Ack         *dhcpv4.DHCPv4
	NetIf       *net.Interface
	DHCPOptions config.DHCPOptions
	Mtu         int
	RouteList   []config.Route
}

// Name returns back the name of the address method.
func (d *DHCP4) Name() string {
	return "dhcp4"
}

// Link returns the underlying net.Interface that this address
// method is configured for.
func (d *DHCP4) Link() *net.Interface {
	return d.NetIf
}

// Discover handles the DHCP client exchange stores the DHCP Ack.
func (d *DHCP4) Discover(ctx context.Context, logger *log.Logger, link *net.Interface) error {
	d.NetIf = link
	err := d.discover(ctx, logger)

	return err
}

// Address returns back the IP address from the received DHCP offer.
func (d *DHCP4) Address() *net.IPNet {
	return &net.IPNet{
		IP:   d.Ack.YourIPAddr,
		Mask: d.Mask(),
	}
}

// Mask returns the netmask from the DHCP offer.
func (d *DHCP4) Mask() net.IPMask {
	return d.Ack.SubnetMask()
}

// MTU returs the MTU size from the DHCP offer.
func (d *DHCP4) MTU() uint32 {
	mtuReturn := uint32(d.NetIf.MTU)

	if d.Ack != nil {
		// TODO do we need to implement dhcpv4.GetUint32 upstream?
		mtu, err := dhcpv4.GetUint16(dhcpv4.OptionInterfaceMTU, d.Ack.Options)
		if err == nil {
			mtuReturn = uint32(mtu)
		}
	}

	// override with any non-zero Mtu value passed into the dhcp object
	if uint32(d.Mtu) > 0 {
		mtuReturn = uint32(d.Mtu)
	}

	return mtuReturn
}

// TTL denotes how long a DHCP offer is valid for.
func (d *DHCP4) TTL() time.Duration {
	if d.Ack == nil {
		return 0
	}

	return d.Ack.IPAddressLeaseTime(time.Minute * 30)
}

// Family qualifies the address as ipv4 or ipv6.
func (d *DHCP4) Family() int {
	return unix.AF_INET
}

// Scope sets the address scope.
func (d *DHCP4) Scope() uint8 {
	return unix.RT_SCOPE_UNIVERSE
}

// Valid denotes if this address method should be used.
func (d *DHCP4) Valid() bool {
	return d.Ack != nil
}

// Routes aggregates all Routers and ClasslessStaticRoutes retrieved from
// the DHCP offer.
// rfc3442:
//   If the DHCP server returns both a Classless Static Routes option and
//   a Router option, the DHCP client MUST ignore the Router option.
func (d *DHCP4) Routes() (routes []*Route) {
	metric := dhcpReceivedRouteMetric

	if d.DHCPOptions != nil && d.DHCPOptions.RouteMetric() != 0 {
		metric = d.DHCPOptions.RouteMetric()
	}

	defRoute := &net.IPNet{
		IP:   net.IPv4zero,
		Mask: net.IPv4Mask(0, 0, 0, 0),
	}

	for _, router := range d.Ack.Router() {
		routes = append(routes, &Route{
			Destination: defRoute,
			Gateway:     router,
			Metric:      metric,
		})
	}

	// overwrite router option if classless routes were provided.
	if len(d.Ack.ClasslessStaticRoute()) > 0 {
		routes = []*Route{}

		for _, dhcpRoute := range d.Ack.ClasslessStaticRoute() {
			routes = append(routes, &Route{
				Destination: dhcpRoute.Dest,
				Gateway:     dhcpRoute.Router,
				Metric:      metric,
			})
		}
	}

	// append any routes that were provided in config
	for _, route := range d.RouteList {
		_, ipnet, err := net.ParseCIDR(route.Network())
		if err != nil {
			// TODO: we should at least log this failure
			continue
		}

		routes = append(routes, &Route{
			Destination: ipnet,
			Gateway:     net.ParseIP(route.Gateway()),
			Metric:      staticRouteDefaultMetric,
		})
	}

	return routes
}

// Resolvers returns the DNS resolvers from the DHCP offer.
func (d *DHCP4) Resolvers() []net.IP {
	return d.Ack.DNS()
}

// Hostname returns the hostname from the DHCP offer.
func (d *DHCP4) Hostname() (hostname string) {
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
func (d *DHCP4) discover(ctx context.Context, logger *log.Logger) error {
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
	clientOpts := []nclient4.ClientOpt{}

	if d.Offer != nil {
		// do not use broadcast, but send the packet to DHCP server directly
		addr, err := net.ResolveUDPAddr("udp", d.Offer.ServerIPAddr.String()+":67")
		if err != nil {
			return err
		}

		// by default it's set to 0.0.0.0 which actually breaks lease renew
		d.Offer.ClientIPAddr = d.Offer.YourIPAddr

		clientOpts = append(clientOpts, nclient4.WithServerAddr(addr))
	}

	// TODO expose this ( nclient4.WithDebugLogger() ) with some
	// debug logging option
	cli, err := nclient4.New(d.NetIf.Name, clientOpts...)
	if err != nil {
		return err
	}

	//nolint:errcheck
	defer cli.Close()

	var lease *nclient4.Lease

	if d.Offer != nil {
		lease, err = cli.RequestFromOffer(ctx, d.Offer, mods...)
	} else {
		lease, err = cli.Request(ctx, mods...)
	}

	if err != nil {
		// TODO: Make this a well defined error so we can make it not fatal
		logger.Printf("failed dhcp request for %q: %v", d.NetIf.Name, err)

		// clear offer if request fails to start with discover sequence next time
		d.Offer = nil

		return err
	}

	logger.Printf("DHCP ACK on %q: %s", d.NetIf.Name, collapseSummary(lease.ACK.Summary()))

	d.Ack = lease.ACK
	d.Offer = lease.Offer

	return err
}

func collapseSummary(summary string) string {
	lines := strings.Split(summary, "\n")[1:]

	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}

	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return strings.Join(lines, ", ")
}
