// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package bgp contains translations between Talos network resources and GoBGP types.
package bgp

import (
	"fmt"
	"net/netip"
	"strings"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// Peer is a BGP neighbor with its runtime address resolved.
type Peer struct {
	Config    network.BGPNeighborConfigSpec
	Address   string
	LinkLocal netip.Addr
	Link      string
	// BindInterface constrains the outbound transport to a VRF or interface.
	BindInterface string
}

// BuildPeer translates a resolved peer into a GoBGP peer.
func BuildPeer(peer Peer, multipath bool) *gobgpapi.Peer {
	result := &gobgpapi.Peer{
		Conf: &gobgpapi.PeerConf{
			PeerAsn:         peer.Config.PeerASN,
			NeighborAddress: peer.Address,
		},
		AfiSafis: []*gobgpapi.AfiSafi{
			afiSafi(gobgpapi.Family_AFI_IP, multipath),
			afiSafi(gobgpapi.Family_AFI_IP6, multipath),
		},
	}

	if peer.Config.LocalASN != 0 {
		result.Conf.LocalAsn = peer.Config.LocalASN
		result.Conf.ReplacePeerAsn = true
	}

	if peer.Config.Passive || peer.BindInterface != "" {
		result.Transport = &gobgpapi.Transport{
			PassiveMode:   peer.Config.Passive,
			BindInterface: peer.BindInterface,
		}
	}

	if peer.Config.HoldTime > 0 {
		hold := uint64(peer.Config.HoldTime.Seconds())

		result.Timers = &gobgpapi.Timers{
			Config: &gobgpapi.TimersConfig{
				HoldTime:          hold,
				KeepaliveInterval: hold / 3,
			},
		}
	}

	if peer.Config.BFD != nil {
		result.Bfd = &gobgpapi.BfdPeerConfig{
			Enabled:                  true,
			DesiredMinimumTxInterval: uint32(peer.Config.BFD.TransmitInterval.Microseconds()),
			RequiredMinimumReceive:   uint32(peer.Config.BFD.ReceiveInterval.Microseconds()),
			DetectionMultiplier:      uint32(peer.Config.BFD.DetectMultiplier),
		}
	}

	return result
}

func afiSafi(afi gobgpapi.Family_Afi, multipath bool) *gobgpapi.AfiSafi {
	as := &gobgpapi.AfiSafi{
		Config: &gobgpapi.AfiSafiConfig{
			Family:  &gobgpapi.Family{Afi: afi, Safi: gobgpapi.Family_SAFI_UNICAST},
			Enabled: true,
		},
	}

	if multipath {
		as.UseMultiplePaths = &gobgpapi.UseMultiplePaths{
			Config: &gobgpapi.UseMultiplePathsConfig{Enabled: true},
		}
	}

	return as
}

// BuildOriginatedPath builds a host-route path advertising the given prefix with next-hop self.
func BuildOriginatedPath(prefix netip.Prefix) (*apiutil.Path, error) {
	nlri, err := bgppacket.NewIPAddrPrefix(prefix)
	if err != nil {
		return nil, fmt.Errorf("error building NLRI for %s: %w", prefix, err)
	}

	origin := bgppacket.NewPathAttributeOrigin(0)

	// Originate with the unspecified next-hop so GoBGP applies next-hop-self per peer.
	if prefix.Addr().Is4() {
		nexthop, nhErr := bgppacket.NewPathAttributeNextHop(netip.IPv4Unspecified())
		if nhErr != nil {
			return nil, fmt.Errorf("error building next-hop for %s: %w", prefix, nhErr)
		}

		return &apiutil.Path{
			Family: bgppacket.RF_IPv4_UC,
			Nlri:   nlri,
			Attrs:  []bgppacket.PathAttributeInterface{origin, nexthop},
		}, nil
	}

	mpReach, mpErr := bgppacket.NewPathAttributeMpReachNLRI(
		bgppacket.RF_IPv6_UC,
		[]bgppacket.PathNLRI{{NLRI: nlri}},
		netip.IPv6Unspecified(),
	)
	if mpErr != nil {
		return nil, fmt.Errorf("error building MP_REACH for %s: %w", prefix, mpErr)
	}

	return &apiutil.Path{
		Family: bgppacket.RF_IPv6_UC,
		Nlri:   nlri,
		Attrs:  []bgppacket.PathAttributeInterface{origin, mpReach},
	}, nil
}

// PathNexthop extracts the next-hop address from a path's attributes.
func PathNexthop(path *apiutil.Path) netip.Addr {
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

// RouteSpec builds a RouteSpec for a learned destination and its next-hops.
func RouteSpec(prefix netip.Prefix, nexthops []network.RouteNextHop, source netip.Addr, table nethelpers.RoutingTable) network.RouteSpecSpec {
	if table == nethelpers.TableUnspec {
		table = nethelpers.TableMain
	}

	spec := network.RouteSpecSpec{
		Family:      addrFamily(prefix.Addr()),
		Destination: prefix,
		Table:       table,
		Protocol:    nethelpers.ProtocolBGP,
		Type:        nethelpers.TypeUnicast,
		Scope:       nethelpers.ScopeGlobal,
		ConfigLayer: network.ConfigOperator,
	}

	if source.IsValid() && source.Is4() == prefix.Addr().Is4() {
		spec.Source = source
	}

	if len(nexthops) == 1 {
		spec.Gateway = nexthops[0].Gateway
		spec.OutLinkName = nexthops[0].OutLinkName
	} else {
		spec.NextHops = nexthops
	}

	return spec
}

// PeerStatus translates a GoBGP peer into a BGPPeerStatusSpec.
func PeerStatus(peer *gobgpapi.Peer, localASN uint32) network.BGPPeerStatusSpec {
	conf := peer.GetConf()
	state := peer.GetState()

	id := conf.GetNeighborAddress()
	if id == "" {
		id = "link/" + conf.GetNeighborInterface()
	}

	spec := network.BGPPeerStatusSpec{
		Peer:     id,
		LocalASN: localASN,
		PeerASN:  conf.GetPeerAsn(),
		State:    SessionState(state.GetSessionState()),
	}

	if conf.GetLocalAsn() != 0 {
		spec.LocalASN = conf.GetLocalAsn()
	}

	if spec.PeerASN == 0 {
		spec.PeerASN = state.GetPeerAsn()
	}

	if routerID, err := netip.ParseAddr(state.GetRouterId()); err == nil {
		spec.RouterID = routerID
	}

	if uptime := peer.GetTimers().GetState().GetUptime(); uptime != nil {
		spec.Since = uptime.AsTime()
	}

	for _, family := range peer.GetAfiSafis() {
		spec.Received += uint32(family.GetState().GetReceived())
		spec.Accepted += uint32(family.GetState().GetAccepted())
		spec.Advertised += uint32(family.GetState().GetAdvertised())
	}

	if bfd := state.GetBfdState(); bfd != nil && bfd.GetSessionState() != gobgpapi.BfdSessionState_BFD_SESSION_STATE_UNSPECIFIED {
		spec.BFDState = strings.ToLower(strings.TrimPrefix(bfd.GetSessionState().String(), "BFD_SESSION_STATE_"))
	}

	return spec
}

// SessionState converts a GoBGP session state into its Talos representation.
func SessionState(state gobgpapi.PeerState_SessionState) nethelpers.BGPSessionState {
	switch state {
	case gobgpapi.PeerState_SESSION_STATE_IDLE:
		return nethelpers.BGPSessionStateIdle
	case gobgpapi.PeerState_SESSION_STATE_CONNECT:
		return nethelpers.BGPSessionStateConnect
	case gobgpapi.PeerState_SESSION_STATE_ACTIVE:
		return nethelpers.BGPSessionStateActive
	case gobgpapi.PeerState_SESSION_STATE_OPENSENT:
		return nethelpers.BGPSessionStateOpenSent
	case gobgpapi.PeerState_SESSION_STATE_OPENCONFIRM:
		return nethelpers.BGPSessionStateOpenConfirm
	case gobgpapi.PeerState_SESSION_STATE_ESTABLISHED:
		return nethelpers.BGPSessionStateEstablished
	case gobgpapi.PeerState_SESSION_STATE_UNSPECIFIED:
		return nethelpers.BGPSessionStateUnknown
	default:
		return nethelpers.BGPSessionStateUnknown
	}
}

func addrFamily(addr netip.Addr) nethelpers.Family {
	if addr.Is4() || addr.Is4In6() {
		return nethelpers.FamilyInet4
	}

	return nethelpers.FamilyInet6
}

// ServerKey returns a deterministic representation of server-level configuration.
func ServerKey(localASN uint32, routerID netip.Addr, multipath bool, maxPaths uint8, vrf string, table nethelpers.RoutingTable, listenPort int32) string {
	return fmt.Sprintf("asn=%d;router=%s;multipath=%t;maxpaths=%d;vrf=%s;table=%s;listen=%d;", localASN, routerID, multipath, maxPaths, vrf, table, listenPort)
}

// PeerKey returns a deterministic representation of a peer's configuration.
func PeerKey(peer Peer) string {
	var builder strings.Builder

	fmt.Fprintf(
		&builder,
		"%s/%d/%d/%t/%s/%s",
		peer.Address,
		peer.Config.PeerASN,
		peer.Config.LocalASN,
		peer.Config.Passive,
		peer.BindInterface,
		peer.Config.HoldTime,
	)

	if peer.Config.BFD != nil {
		fmt.Fprintf(
			&builder,
			"bfd[%s/%s/%d]",
			peer.Config.BFD.TransmitInterval,
			peer.Config.BFD.ReceiveInterval,
			peer.Config.BFD.DetectMultiplier,
		)
	}

	return builder.String()
}
