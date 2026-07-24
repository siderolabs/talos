// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

package mgmt

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"slices"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/jsimonetti/rtnetlink/v2"
	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
	"golang.org/x/sys/unix"
)

const fabricRetryInterval = 5 * time.Second

// fabricZebra plays the "zebra" data-plane role for the test fabric peer: it watches gobgp best paths
// and programs learned node prefixes (e.g. a node's BGP loopback) into the host kernel FIB, so talosctl
// (and the other nodes, via the host) can reach a node purely via BGP. With natCIDR set (full-CLOS) it
// also enables IP forwarding and masquerades that CIDR so the BGP-only nodes reach host services + the
// internet. Linux-only (rtnetlink + netfilter). It blocks until ctx is done.
//
// Egress is resolved per-path from the peer the route was learned over (so it works across many fabric
// bridges, not just one).
func fabricZebra(ctx context.Context, srv *gobgpsrv.BgpServer, ifaces []string, natCIDR string) error {
	if err := fabricSetupNAT(natCIDR); err != nil {
		return err
	}

	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("error dialing rtnetlink: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	reconcile := make(chan struct{}, 1)

	signal := func() {
		select {
		case reconcile <- struct{}{}:
		default:
		}
	}

	if err = srv.WatchEvent(ctx, gobgpsrv.WatchEventMessageCallbacks{
		OnBestPath: func([]*apiutil.Path, time.Time) { signal() },
	}, gobgpsrv.WatchBestPath(true)); err != nil {
		return fmt.Errorf("error watching BGP events: %w", err)
	}

	fabricIfaces := fabricInterfaceIndices(ifaces)

	installed, err := fabricLoadOwnedRoutes(conn, fabricIfaces)
	if err != nil {
		return fmt.Errorf("error loading existing fabric routes: %w", err)
	}

	defer fabricCleanupRoutes(conn, installed)

	ticker := time.NewTicker(fabricRetryInterval)
	defer ticker.Stop()

	for {
		fabricReconcileRoutes(srv, conn, ifaces, installed)

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-reconcile:
		}
	}
}

// fabricSetupNAT enables host forwarding and masquerades the node loopback CIDR, so the BGP-only nodes
// (whose only address is a loopback in this CIDR) can reach the host services and the internet. No-op
// when natCIDR is empty (the single-bridge reachability mode reaches the host directly).
func fabricSetupNAT(natCIDR string) error {
	if natCIDR == "" {
		return nil
	}

	if _, err := netip.ParsePrefix(natCIDR); err != nil {
		return fmt.Errorf("invalid --bgp-nat-cidr %q: %w", natCIDR, err)
	}

	for _, knob := range []string{
		"/proc/sys/net/ipv4/ip_forward",
		"/proc/sys/net/ipv4/conf/all/forwarding",
		"/proc/sys/net/ipv6/conf/all/forwarding",
	} {
		if err := os.WriteFile(knob, []byte("1\n"), 0o644); err != nil {
			return fmt.Errorf("enabling %s: %w", knob, err)
		}
	}

	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("init iptables: %w", err)
	}

	// masquerade node loopback traffic leaving the fabric (host services + internet); allow forwarding
	// both ways (the host may default to DROP on FORWARD, e.g. when Docker is installed).
	rules := []struct {
		table, chain string
		spec         []string
	}{
		{"nat", "POSTROUTING", []string{"-s", natCIDR, "!", "-d", natCIDR, "-j", "MASQUERADE"}},
		{"filter", "FORWARD", []string{"-s", natCIDR, "-j", "ACCEPT"}},
		{"filter", "FORWARD", []string{"-d", natCIDR, "-j", "ACCEPT"}},
	}

	for _, r := range rules {
		if err := ipt.AppendUnique(r.table, r.chain, r.spec...); err != nil {
			return fmt.Errorf("adding %s/%s rule: %w", r.table, r.chain, err)
		}
	}

	return nil
}

// fabricHop is a single resolved next-hop (link-local gateway + egress interface) for a learned route.
type fabricHop struct {
	gw  netip.Addr
	oif int
}

// fabricReconcileRoutes programs learned best paths into the host FIB and removes withdrawn ones. A
// destination advertised by multiple nodes (e.g. an anycast control-plane VIP shared by every CP) yields
// multiple best paths (gobgp runs with UseMultiplePaths) and is installed as an ECMP multipath route.
//
//nolint:gocyclo
func fabricReconcileRoutes(
	srv *gobgpsrv.BgpServer,
	conn *rtnetlink.Conn,
	ifaces []string,
	installed map[netip.Prefix]rtnetlink.RouteMessage,
) {
	desired := map[netip.Prefix][]fabricHop{}

	for _, family := range []bgppacket.Family{bgppacket.RF_IPv4_UC, bgppacket.RF_IPv6_UC} {
		err := srv.ListPath(apiutil.ListPathRequest{
			TableType: gobgpapi.TableType_TABLE_TYPE_GLOBAL,
			Family:    family,
		}, func(prefix bgppacket.NLRI, paths []*apiutil.Path) {
			dst, parseErr := netip.ParsePrefix(prefix.String())
			if parseErr != nil {
				return
			}

			// The provisioner host only needs direct access to node identities and advertised VIPs.
			// Keep broader workload routes in the fabric RIB without replacing an unrelated host
			// route such as the provisioner's own Kubernetes PodCIDR.
			if !fabricHostRouteEligible(dst) {
				return
			}

			for _, path := range paths {
				// only install routes learned from the node, skipping our own originated prefixes.
				if !path.Best || path.Withdrawal || !path.PeerAddress.IsValid() {
					continue
				}

				nh := fabricNexthop(path)
				if !nh.IsValid() || nh.IsUnspecified() {
					continue
				}

				oif := fabricEgress(path, ifaces)
				if oif == 0 {
					continue
				}

				hop := fabricHop{gw: nh, oif: oif}

				// gobgp may report the same next-hop more than once; keep the set unique.
				if !slices.Contains(desired[dst], hop) {
					desired[dst] = append(desired[dst], hop)
				}
			}
		})
		if err != nil {
			continue
		}
	}

	for dst, hops := range desired {
		if _, ok := installed[dst]; !ok {
			occupied, err := fabricRouteExists(conn, dst)
			if err != nil || occupied {
				continue
			}
		}

		// re-install every reconcile (Replace is idempotent): the next-hop set changes as nodes
		// advertising an anycast VIP come and go (control-plane failover).
		route, err := fabricInstallRoute(conn, dst, hops)
		if err == nil {
			installed[dst] = route
		}
	}

	for dst, route := range installed {
		if _, ok := desired[dst]; ok {
			continue
		}

		fabricDeleteRoute(conn, route)
		delete(installed, dst)
	}
}

func fabricRouteExists(conn *rtnetlink.Conn, dst netip.Prefix) (bool, error) {
	routes, err := conn.Route.List()
	if err != nil {
		return false, err
	}

	for _, route := range routes {
		if route.Attributes.Table == unix.RT_TABLE_MAIN &&
			route.DstLength == uint8(dst.Bits()) &&
			route.Attributes.Dst.Equal(dst.Addr().AsSlice()) {
			return true, nil
		}
	}

	return false, nil
}

func fabricInterfaceIndices(ifaces []string) map[uint32]struct{} {
	result := make(map[uint32]struct{}, len(ifaces))

	for _, name := range ifaces {
		iface, err := net.InterfaceByName(name)
		if err == nil {
			result[uint32(iface.Index)] = struct{}{}
		}
	}

	return result
}

func fabricLoadOwnedRoutes(
	conn *rtnetlink.Conn,
	ifaces map[uint32]struct{},
) (map[netip.Prefix]rtnetlink.RouteMessage, error) {
	routes, err := conn.Route.List()
	if err != nil {
		return nil, err
	}

	return fabricOwnedRouteInventory(routes, ifaces), nil
}

func fabricOwnedRouteInventory(
	routes []rtnetlink.RouteMessage,
	ifaces map[uint32]struct{},
) map[netip.Prefix]rtnetlink.RouteMessage {
	result := map[netip.Prefix]rtnetlink.RouteMessage{}

	for _, route := range routes {
		prefix, ok := fabricOwnedRoutePrefix(route, ifaces)
		if ok {
			result[prefix] = route
		}
	}

	return result
}

func fabricOwnedRoutePrefix(
	route rtnetlink.RouteMessage,
	ifaces map[uint32]struct{},
) (netip.Prefix, bool) {
	if route.Protocol != unix.RTPROT_BGP || route.Attributes.Table != unix.RT_TABLE_MAIN {
		return netip.Prefix{}, false
	}

	address, ok := netip.AddrFromSlice(route.Attributes.Dst)
	if !ok {
		return netip.Prefix{}, false
	}

	prefix := netip.PrefixFrom(address.Unmap(), int(route.DstLength)).Masked()
	if !fabricHostRouteEligible(prefix) || !fabricRouteUsesInterfaces(route, ifaces) {
		return netip.Prefix{}, false
	}

	return prefix, true
}

func fabricRouteUsesInterfaces(route rtnetlink.RouteMessage, ifaces map[uint32]struct{}) bool {
	if len(route.Attributes.Multipath) == 0 {
		_, ok := ifaces[route.Attributes.OutIface]

		return ok
	}

	for _, hop := range route.Attributes.Multipath {
		if _, ok := ifaces[hop.Hop.IfIndex]; !ok {
			return false
		}
	}

	return true
}

// fabricEgress resolves the egress interface index for a learned path: the peer's link-local next-hop is
// reachable over the interface carried in the peer address zone (set by gobgp on the dynamic-neighbor
// session). Falls back to the single configured interface when the zone is absent.
func fabricEgress(path *apiutil.Path, ifaces []string) int {
	zone := path.PeerAddress.Zone()
	if zone == "" && len(ifaces) > 0 {
		zone = ifaces[0]
	}

	if zone == "" {
		return 0
	}

	ifi, err := net.InterfaceByName(zone)
	if err != nil {
		return 0
	}

	return ifi.Index
}

// fabricInstallRoute installs a host route to dst via the given next-hop(s), using RTA_VIA for a
// cross-family (IPv4-dst / IPv6-link-local-nh) next-hop (RFC 8950). A single hop installs a plain route;
// multiple hops install an ECMP multipath route (RTA_MULTIPATH), one next-hop per advertising node.
func fabricInstallRoute(conn *rtnetlink.Conn, dst netip.Prefix, hops []fabricHop) (rtnetlink.RouteMessage, error) {
	if len(hops) == 0 {
		return rtnetlink.RouteMessage{}, nil
	}

	family := uint8(unix.AF_INET)
	if dst.Addr().Is6() {
		family = unix.AF_INET6
	}

	attrs := rtnetlink.RouteAttributes{
		Dst:   dst.Addr().AsSlice(),
		Table: unix.RT_TABLE_MAIN,
	}

	crossFamily := func(gw netip.Addr) bool { return dst.Addr().Is4() && gw.Is6() }

	if len(hops) == 1 {
		attrs.OutIface = uint32(hops[0].oif)

		if crossFamily(hops[0].gw) {
			attrs.Via = &rtnetlink.RouteVia{Family: unix.AF_INET6, Addr: hops[0].gw.AsSlice()}
		} else {
			attrs.Gateway = hops[0].gw.AsSlice()
		}
	} else {
		attrs.Multipath = make([]rtnetlink.NextHop, len(hops))

		for i, h := range hops {
			hop := rtnetlink.NextHop{Hop: rtnetlink.RTNextHop{IfIndex: uint32(h.oif)}}

			if crossFamily(h.gw) {
				hop.Via = &rtnetlink.RouteVia{Family: unix.AF_INET6, Addr: h.gw.AsSlice()}
			} else {
				hop.Gateway = h.gw.AsSlice()
			}

			attrs.Multipath[i] = hop
		}
	}

	route := rtnetlink.RouteMessage{
		Family:     family,
		DstLength:  uint8(dst.Bits()),
		Protocol:   unix.RTPROT_BGP,
		Scope:      unix.RT_SCOPE_UNIVERSE,
		Type:       unix.RTN_UNICAST,
		Attributes: attrs,
	}

	return route, conn.Route.Replace(&route)
}

func fabricDeleteRoute(conn *rtnetlink.Conn, route rtnetlink.RouteMessage) {
	conn.Route.Delete(&route) //nolint:errcheck
}

func fabricCleanupRoutes(conn *rtnetlink.Conn, installed map[netip.Prefix]rtnetlink.RouteMessage) {
	for _, route := range installed {
		fabricDeleteRoute(conn, route)
	}
}

// fabricNexthop extracts the installable next-hop from a learned path, preferring the link-local
// next-hop (RFC 8950 unnumbered).
func fabricNexthop(path *apiutil.Path) netip.Addr {
	for _, attr := range path.Attrs {
		switch a := attr.(type) {
		case *bgppacket.PathAttributeNextHop:
			return a.Value
		case *bgppacket.PathAttributeMpReachNLRI:
			if a.LinkLocalNexthop.IsValid() && a.LinkLocalNexthop.IsLinkLocalUnicast() {
				return a.LinkLocalNexthop
			}

			if a.Nexthop.IsValid() && !a.Nexthop.IsUnspecified() {
				return a.Nexthop
			}

			return a.LinkLocalNexthop
		}
	}

	return netip.Addr{}
}
