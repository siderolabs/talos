// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mgmt

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/mdlayher/ndp"
	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
	"github.com/spf13/cobra"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const fabricRAInterval = 10 * time.Second

var fabricAllNodes = netip.MustParseAddr("ff02::1")

var bgpLaunchCmdFlags struct {
	addr          string
	neighborRange string
	vrfNeighbor   string
	advertise     string
	asn           uint32
	peerASN       uint32
	unnumbered    bool
	zebra         bool
	ifaces        []string
	natCIDR       string
}

// bgpLaunchCmd represents the bgp-launch command: an embedded gobgp speaker acting as a fabric
// leaf/ToR peer for integration testing of native BGP.
var bgpLaunchCmd = &cobra.Command{
	Use:    "bgp-launch",
	Short:  "Internal command used by the QEMU provisioner",
	Long:   ``,
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if bgpLaunchCmdFlags.unnumbered {
			return runUnnumberedFabricPeer(cmd.Context())
		}

		routerID, err := netip.ParseAddr(bgpLaunchCmdFlags.addr)
		if err != nil {
			return fmt.Errorf("invalid --bgp-addr: %w", err)
		}

		lvl := new(slog.LevelVar)
		lvl.Set(slog.LevelInfo)

		// log to stderr so the provisioner captures fabric-peer activity in bgp.log
		srv := gobgpsrv.NewBgpServer(gobgpsrv.LoggerOption(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})), lvl))

		go srv.Serve()

		ctx := cmd.Context()

		if err = srv.StartBgp(ctx, &gobgpapi.StartBgpRequest{
			Global: &gobgpapi.Global{
				Asn:              bgpLaunchCmdFlags.asn,
				RouterId:         routerID.String(),
				ListenPort:       constants.BGPDefaultPort,
				UseMultiplePaths: true,
			},
		}); err != nil {
			return fmt.Errorf("error starting BGP: %w", err)
		}

		const peerGroupName = "nodes"

		if err = srv.AddPeerGroup(ctx, &gobgpapi.AddPeerGroupRequest{
			PeerGroup: &gobgpapi.PeerGroup{
				Conf: &gobgpapi.PeerGroupConf{
					PeerGroupName: peerGroupName,
					PeerAsn:       bgpLaunchCmdFlags.peerASN,
				},
				AfiSafis: []*gobgpapi.AfiSafi{
					bgpLaunchAfiSafi(gobgpapi.Family_AFI_IP),
					bgpLaunchAfiSafi(gobgpapi.Family_AFI_IP6),
				},
			},
		}); err != nil {
			return fmt.Errorf("error adding peer group: %w", err)
		}

		if bgpLaunchCmdFlags.vrfNeighbor != "" {
			vrfNeighbor, parseErr := netip.ParseAddr(bgpLaunchCmdFlags.vrfNeighbor)
			if parseErr != nil {
				return fmt.Errorf("invalid --bgp-vrf-neighbor: %w", parseErr)
			}

			// This address is inside the dynamic-neighbor range, but GoBGP matches an explicitly
			// configured peer before consulting dynamic ranges. The static peer actively dials the
			// passive Talos VRF neighbor, directly exercising the VRF-bound listener's accept path.
			if err = srv.AddPeer(ctx, &gobgpapi.AddPeerRequest{
				Peer: bgpLaunchActivePeer(vrfNeighbor, bgpLaunchCmdFlags.peerASN),
			}); err != nil {
				return fmt.Errorf("error adding active VRF test peer: %w", err)
			}
		}

		if err = srv.AddDynamicNeighbor(ctx, &gobgpapi.AddDynamicNeighborRequest{
			DynamicNeighbor: &gobgpapi.DynamicNeighbor{
				Prefix:    bgpLaunchCmdFlags.neighborRange,
				PeerGroup: peerGroupName,
			},
		}); err != nil {
			return fmt.Errorf("error adding dynamic neighbor: %w", err)
		}

		if bgpLaunchCmdFlags.advertise != "" {
			prefix, parseErr := netip.ParsePrefix(bgpLaunchCmdFlags.advertise)
			if parseErr != nil {
				return fmt.Errorf("invalid --bgp-advertise: %w", parseErr)
			}

			if err = bgpLaunchAdvertise(srv, prefix); err != nil {
				return fmt.Errorf("error advertising %s: %w", prefix, err)
			}
		}

		<-ctx.Done()

		srv.Stop()

		return nil
	},
}

// runUnnumberedFabricPeer runs the test fabric peer in unnumbered mode: it peers with each node over
// IPv6 link-local on every --bgp-interface (via a dynamic neighbor over fe80::/10), sends Router
// Advertisements so the nodes discover the host's link-local and dial in, and advertises a return
// route. This core (gobgp + RA) is cross-platform.
//
// With --bgp-zebra it additionally programs learned node prefixes into the host kernel FIB (the "zebra"
// role, so talosctl can reach a node purely via BGP) — that part is Linux-only (see fabricZebra).
//
//nolint:gocyclo
func runUnnumberedFabricPeer(ctx context.Context) error {
	routerID, err := netip.ParseAddr(bgpLaunchCmdFlags.addr)
	if err != nil {
		return fmt.Errorf("invalid --bgp-addr: %w", err)
	}

	if len(bgpLaunchCmdFlags.ifaces) == 0 {
		return fmt.Errorf("--bgp-interface is required for unnumbered mode")
	}

	lvl := new(slog.LevelVar)
	lvl.Set(slog.LevelInfo)

	srv := gobgpsrv.NewBgpServer(gobgpsrv.LoggerOption(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})), lvl))

	go srv.Serve()

	defer srv.Stop()

	if err = srv.StartBgp(ctx, &gobgpapi.StartBgpRequest{
		Global: &gobgpapi.Global{
			Asn:              bgpLaunchCmdFlags.asn,
			RouterId:         routerID.String(),
			ListenPort:       constants.BGPDefaultPort,
			UseMultiplePaths: true,
		},
	}); err != nil {
		return fmt.Errorf("error starting BGP: %w", err)
	}

	// Accept any link-local peer via a dynamic neighbor: each node dials in actively (it discovers us
	// from our Router Advertisements). We don't resolve a specific peer; gobgp still negotiates the
	// extended-nexthop capability (RFC 8950) on the IPv6-link-local session so IPv4 prefixes exchange
	// over the IPv6 next-hop.
	//
	// PeerAsn 0 skips ASN negotiation (gobgp's equivalent of "remote-as external"): in CLOS each node
	// has its own ASN (distinct ASNs are required so the fabric peer can re-advertise one node's routes
	// to another without the AS_PATH loop check rejecting them), and the fabric must accept all of them.
	const peerGroupName = "nodes"

	if err = srv.AddPeerGroup(ctx, &gobgpapi.AddPeerGroupRequest{
		PeerGroup: &gobgpapi.PeerGroup{
			Conf: &gobgpapi.PeerGroupConf{
				PeerGroupName: peerGroupName,
				PeerAsn:       0,
			},
			AfiSafis: []*gobgpapi.AfiSafi{
				bgpLaunchAfiSafi(gobgpapi.Family_AFI_IP),
				bgpLaunchAfiSafi(gobgpapi.Family_AFI_IP6),
			},
			// enable BFD so the node's BFD-enabled neighbor brings up a session (BFD needs both ends).
			// 300ms tx/rx, multiplier 3 — matches the node-side test config; BFD negotiates the rest.
			Bfd: &gobgpapi.BfdPeerConfig{
				Enabled:                  true,
				DesiredMinimumTxInterval: 300_000,
				RequiredMinimumReceive:   300_000,
				DetectionMultiplier:      3,
			},
		},
	}); err != nil {
		return fmt.Errorf("error adding peer group: %w", err)
	}

	if err = srv.AddDynamicNeighbor(ctx, &gobgpapi.AddDynamicNeighborRequest{
		DynamicNeighbor: &gobgpapi.DynamicNeighbor{
			Prefix:    "fe80::/10",
			PeerGroup: peerGroupName,
		},
	}); err != nil {
		return fmt.Errorf("error adding dynamic neighbor: %w", err)
	}

	if bgpLaunchCmdFlags.advertise != "" {
		prefix, parseErr := netip.ParsePrefix(bgpLaunchCmdFlags.advertise)
		if parseErr != nil {
			return fmt.Errorf("invalid --bgp-advertise: %w", parseErr)
		}

		if err = bgpLaunchAdvertise(srv, prefix); err != nil {
			return fmt.Errorf("error advertising %s: %w", prefix, err)
		}
	}

	// send Router Advertisements on every fabric interface so the nodes discover the host's link-local.
	// fabricSendRAs resolves the interface in-loop and retries, so this works even though the per-uplink
	// CNI bridges only appear once the nodes attach (CreateBGP runs before the nodes are created).
	for _, name := range bgpLaunchCmdFlags.ifaces {
		go fabricSendRAs(ctx, name)
	}

	if bgpLaunchCmdFlags.zebra {
		// host FIB programming + NAT (Linux-only); blocks until ctx is done.
		return fabricZebra(ctx, srv, bgpLaunchCmdFlags.ifaces, bgpLaunchCmdFlags.natCIDR)
	}

	<-ctx.Done()

	return nil
}

func bgpLaunchAfiSafi(afi gobgpapi.Family_Afi) *gobgpapi.AfiSafi {
	return &gobgpapi.AfiSafi{
		Config: &gobgpapi.AfiSafiConfig{
			Family:  &gobgpapi.Family{Afi: afi, Safi: gobgpapi.Family_SAFI_UNICAST},
			Enabled: true,
		},
	}
}

func bgpLaunchActivePeer(address netip.Addr, peerASN uint32) *gobgpapi.Peer {
	return &gobgpapi.Peer{
		Conf: &gobgpapi.PeerConf{
			NeighborAddress: address.String(),
			PeerAsn:         peerASN,
		},
		AfiSafis: []*gobgpapi.AfiSafi{
			bgpLaunchAfiSafi(gobgpapi.Family_AFI_IP),
			bgpLaunchAfiSafi(gobgpapi.Family_AFI_IP6),
		},
		Timers: &gobgpapi.Timers{
			Config: &gobgpapi.TimersConfig{
				// The peer starts before the integration test configures the reserved guest address.
				ConnectRetry: 1,
			},
		},
	}
}

// bgpLaunchAdvertise originates a prefix with next-hop-self so peers can resolve it.
func bgpLaunchAdvertise(srv *gobgpsrv.BgpServer, prefix netip.Prefix) error {
	nlri, err := bgppacket.NewIPAddrPrefix(prefix)
	if err != nil {
		return err
	}

	origin := bgppacket.NewPathAttributeOrigin(0)

	var attrs []bgppacket.PathAttributeInterface

	family := bgppacket.RF_IPv4_UC

	if prefix.Addr().Is4() {
		nexthop, nhErr := bgppacket.NewPathAttributeNextHop(netip.IPv4Unspecified())
		if nhErr != nil {
			return nhErr
		}

		attrs = []bgppacket.PathAttributeInterface{origin, nexthop}
	} else {
		mpReach, mpErr := bgppacket.NewPathAttributeMpReachNLRI(bgppacket.RF_IPv6_UC, []bgppacket.PathNLRI{{NLRI: nlri}}, netip.IPv6Unspecified())
		if mpErr != nil {
			return mpErr
		}

		family = bgppacket.RF_IPv6_UC
		attrs = []bgppacket.PathAttributeInterface{origin, mpReach}
	}

	_, err = srv.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{{Family: family, Nlri: nlri, Attrs: attrs}}})

	return err
}

// fabricSendRAs periodically emits IPv6 Router Advertisements on the named interface (router-lifetime 0:
// presence / link-local discovery only, not a default gateway) so unnumbered peers learn the host's
// link-local. The interface is resolved in-loop and retried, so it tolerates the interface appearing
// after the peer starts (the per-uplink CNI bridges are created when the nodes attach).
func fabricSendRAs(ctx context.Context, name string) {
	ticker := time.NewTicker(fabricRAInterval)
	defer ticker.Stop()

	for {
		func() {
			ifi, err := net.InterfaceByName(name)
			if err != nil {
				return
			}

			rconn, _, err := ndp.Listen(ifi, ndp.LinkLocal)
			if err != nil {
				return
			}

			defer rconn.Close() //nolint:errcheck

			ra := &ndp.RouterAdvertisement{
				CurrentHopLimit: 64,
				RouterLifetime:  0,
				Options: []ndp.Option{
					&ndp.LinkLayerAddress{
						Direction: ndp.Source,
						Addr:      ifi.HardwareAddr,
					},
				},
			}

			_ = rconn.WriteTo(ra, nil, fabricAllNodes) //nolint:errcheck
		}()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func init() {
	bgpLaunchCmd.Flags().StringVar(&bgpLaunchCmdFlags.addr, "bgp-addr", "", "fabric peer listen/router-id address")
	bgpLaunchCmd.Flags().StringVar(&bgpLaunchCmdFlags.neighborRange, "bgp-neighbor-range", "", "CIDR range accepted as dynamic neighbors")
	bgpLaunchCmd.Flags().StringVar(&bgpLaunchCmdFlags.vrfNeighbor, "bgp-vrf-neighbor", "", "static neighbor dialed for the inbound VRF listener test")
	bgpLaunchCmd.Flags().StringVar(&bgpLaunchCmdFlags.advertise, "bgp-advertise", "", "prefix to advertise to nodes")
	bgpLaunchCmd.Flags().Uint32Var(&bgpLaunchCmdFlags.asn, "bgp-asn", 65000, "fabric ASN")
	bgpLaunchCmd.Flags().Uint32Var(&bgpLaunchCmdFlags.peerASN, "bgp-peer-asn", 65001, "expected node (peer) ASN")
	bgpLaunchCmd.Flags().BoolVar(&bgpLaunchCmdFlags.unnumbered, "bgp-unnumbered", false, "peer unnumbered over --bgp-interface(s) and send Router Advertisements")
	bgpLaunchCmd.Flags().BoolVar(&bgpLaunchCmdFlags.zebra, "bgp-zebra", false, "program learned routes into the host FIB (zebra role); Linux-only")
	bgpLaunchCmd.Flags().StringArrayVar(&bgpLaunchCmdFlags.ifaces, "bgp-interface", nil, "host interface(s) for unnumbered peering (repeatable)")
	bgpLaunchCmd.Flags().StringVar(&bgpLaunchCmdFlags.natCIDR, "bgp-nat-cidr", "", "node loopback CIDR to IP-forward + masquerade (full-CLOS, with --bgp-zebra); Linux-only")

	addCommand(bgpLaunchCmd)
}
