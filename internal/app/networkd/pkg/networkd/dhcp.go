/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"context"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/jsimonetti/rtnetlink"
	"golang.org/x/sys/unix"
)

type DHCP struct {
	Ack   *dhcpv4.DHCPv4
	Index int
}

func (d *DHCP) Configure(conn *rtnetlink.Conn, idx int) error {
	d.Index = idx

	msg, err := conn.Link.Get(uint32(d.Index))
	if err != nil {
		log.Println("failed dhcp.link.get")
		return err
	}

	if err = d.Request(msg.Attributes.Name); err != nil {
		log.Println("failed to get dhcp ack")
		return err
	}

	s, err := d.Static()
	if err != nil {
		log.Println("failed dhcp static generate")
		return err
	}

	if err = s.Configure(conn, d.Index); err != nil {
		log.Println("failed configure")
		return err
	}

	// Handle all the additional routing
	var routes []*Route
	def, err := d.DefaultRoute()
	if err != nil {
		log.Println("failed to get default route")
		return err
	}
	routes = append(routes, def)

	ackRoutes, err := d.Routes()
	if err != nil {
		log.Println("failed to get additional routes")
		return err
	}
	routes = append(routes, ackRoutes...)

	for _, r := range routes {
		exists, err := r.Exists(conn)
		if err != nil {
			return err
		}

		if exists {
			continue
		}

		if err = r.Add(conn); err != nil {
			log.Printf("failed add route %+v", r)
			return err
		}
	}

	return err
}

func (d *DHCP) Request(name string) error {
	mods := []dhcpv4.Modifier{
		dhcpv4.WithRequestedOptions(
			dhcpv4.OptionClasslessStaticRoute,
			dhcpv4.OptionDomainNameServer,
			dhcpv4.OptionDNSDomainSearchList,
			// TODO: handle these options
			dhcpv4.OptionHostName,
			dhcpv4.OptionNTPServers,
			dhcpv4.OptionDomainName,
			// May need to add some code upstream for this
			dhcpv4.OptionInterfaceMTU,
		),
	}

	// TODO expose this with some debug logging option
	//cli, err := nclient4.New(name, nclient4.WithDebugLogger())
	cli, err := nclient4.New(name)
	if err != nil {
		log.Println("failed nclient4.new")
		return err
	}
	defer cli.Close()

	_, ack, err := cli.Request(context.Background(), mods...)
	if err != nil {
		// TODO: Make this a well defined error so we can make it not fatal
		log.Println("failed dhcp request")
		return err
	}
	d.Ack = ack

	return err
}

func (d *DHCP) Static() (*Static, error) {
	// Unsure if this is too opinionated or cool
	//
	// We'll use explicit route to router (dst) with scope LINK
	// followed by a default route using router (gw) with scope UNIVERSE
	//
	// Should land us something like this:
	// default via 10.128.0.1 dev ens4 proto dhcp metric 100
	// 10.128.0.1 dev ens4 proto dhcp scope link metric 100
	//                                      ^ being significant
	//
	// from ip-route man:
	// scope global for all gatewayed unicast routes,
	// scope link for direct unicast and broadcast routes,
	// scope host for local routes.
	//
	// rfc defines the routes listed in order of preference, so by
	// pulling [0] for our router, we should be using the preferred one
	ones, _ := d.Ack.SubnetMask().Size()
	_, routeripnet, err := net.ParseCIDR(d.Ack.Router()[0].String() + "/" + strconv.Itoa(ones))
	if err != nil {
		return nil, err
	}

	var search []string
	if d.Ack.DomainSearch() != nil {
		search = d.Ack.DomainSearch().Labels
	}
	// Configure our interfaces with link scope route to router
	s := &Static{
		NetworkInfo: NetworkInfo{
			IP: d.Ack.YourIPAddr,
			Net: &net.IPNet{
				IP:   d.Ack.YourIPAddr.Mask(d.Ack.SubnetMask()),
				Mask: d.Ack.SubnetMask(),
			},
		},
		Route: &Route{
			Dst: &NetworkInfo{
				IP:  d.Ack.Router()[0],
				Net: routeripnet,
			},
			Family:   unix.AF_INET,
			Scope:    unix.RT_SCOPE_LINK,
			Protocol: unix.RTPROT_DHCP,
			Index:    d.Index,
		},
		Resolv: &Resolver{
			Servers: d.Ack.DNS(),
			Search:  search,
		},
	}

	return s, nil
}

func (d *DHCP) DefaultRoute() (*Route, error) {
	// Add additional default route scope universe
	_, routeripnet, err := net.ParseCIDR(d.Ack.Router()[0].String() + "/32")
	if err != nil {
		return nil, err
	}
	r := &Route{
		Gateway: &NetworkInfo{
			IP:  d.Ack.Router()[0],
			Net: routeripnet,
		},
		Family:   unix.AF_INET,
		Scope:    unix.RT_SCOPE_UNIVERSE,
		Protocol: unix.RTPROT_DHCP,
		Index:    d.Index,
	}

	return r, nil
}

func (d *DHCP) Routes() ([]*Route, error) {
	// Add additional link scope routes
	// Maybe set lower priorities? Not sure I've seen multiple routers in a
	// response before
	routes := make([]*Route, 0, len(d.Ack.Router()))

	// Only do stuff if we have more than a single router
	if len(d.Ack.Router()) == 1 {
		return routes, nil
	}

	// Skip the first router returned since we've already added it
	for idx, route := range d.Ack.Router()[1:] {
		_, routeripnet, err := net.ParseCIDR(route.String() + "/32")
		if err != nil {
			return routes, err
		}

		r := &Route{
			Dst: &NetworkInfo{
				IP:  route,
				Net: routeripnet,
			},
			Family:   unix.AF_INET,
			Scope:    unix.RT_SCOPE_LINK, // should this be universe?
			Protocol: unix.RTPROT_DHCP,
			Index:    d.Index,
		}

		routes[idx] = r
	}

	return routes, nil
}

func (d *DHCP) ClasslessRoutes() ([]*Route, error) {
	routes := make([]*Route, 0, len(d.Ack.ClasslessStaticRoute()))
	if len(d.Ack.ClasslessStaticRoute()) == 0 {
		return routes, nil
	}

	// This probably needs some eyes. It hurt my head
	// The only example for this currently is GCE in that
	// it returns the link scoped route for the router
	// and the default route using the router.
	for idx, route := range d.Ack.ClasslessStaticRoute() {
		/*
			143: Classless route to 10.128.0.1/32 via 0.0.0.0
			143: Classless dest: 10.128.0.1/32 router: 0.0.0.0
			143: Classless route to 0.0.0.0/0 via 10.128.0.1
			143: Classless dest: 0.0.0.0/0 router: 10.128.0.1
		*/

		r := &Route{
			Family:   unix.AF_INET,
			Scope:    unix.RT_SCOPE_LINK,
			Protocol: unix.RTPROT_DHCP,
			Index:    d.Index,
		}

		if route.Dest.IP.Equal(net.IPv4zero) {
			// If route.dest is 0.0.0.0 it should be treated as a
			// gateway
			r.Scope = unix.RT_SCOPE_UNIVERSE
		} else {
			r.Dst = &NetworkInfo{
				IP:  route.Dest.IP,
				Net: route.Dest,
			}
		}

		if !route.Router.Equal(net.IPv4zero) {
			_, ipnet, err := net.ParseCIDR(route.Router.String() + "/32")
			if err != nil {
				return routes, err
			}
			r.Gateway = &NetworkInfo{
				IP:  route.Router,
				Net: ipnet,
			}
		}

		routes[idx] = r
	}

	return routes, nil
}

func (d *DHCP) TTL() time.Duration {
	// Set a default lease time of 30m if we didnt get a response
	return d.Ack.IPAddressLeaseTime(time.Minute * 30)
}
