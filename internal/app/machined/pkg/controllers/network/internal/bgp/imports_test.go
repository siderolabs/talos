// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	resourcenetwork "github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestBGPRouteImportLifecycle(t *testing.T) {
	ctx := t.Context()
	fabricPort := freeBGPImportTestPort(t)
	sourceServer := startBGPImportTestServer(t, ctx, 4200000000, "192.0.2.1", -1)
	targetServer := startBGPImportTestServer(t, ctx, 65001, "192.0.2.2", -1)
	fabricServer := startBGPImportTestServer(t, ctx, 65000, "192.0.2.3", fabricPort)
	connectBGPImportTestFabric(t, ctx, targetServer, fabricServer, fabricPort)

	prefix := netip.MustParsePrefix("198.51.100.100/32")
	sourcePath := newBGPImportTestPath(t, prefix, 50)
	originatedPrefix := netip.MustParsePrefix("198.51.100.101/32")
	originatedSourcePath := newBGPImportTestPath(t, originatedPrefix, 60)

	responses, err := sourceServer.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{sourcePath, originatedSourcePath}})
	require.NoError(t, err)
	require.Len(t, responses, 2)

	for _, response := range responses {
		require.NoError(t, response.Error)
	}

	source := internalbgp.NewInstance()
	internalbgp.SetInstanceServerForTest(source, sourceServer)
	source.SetOutputState(nil, nil, 0, netip.Addr{}, 0, false)
	assert.Empty(t, source.Snapshot(ctx).Learned, "installRoutes=false must suppress FIB output")

	source.SetOutputState(nil, nil, 0, netip.Addr{}, 0, true)
	assert.Contains(t, source.Snapshot(ctx).Learned, prefix, "installRoutes=true must restore FIB output")

	source.SetOutputState(nil, nil, 0, netip.Addr{}, 0, false)
	candidates, err := internalbgp.ListImportCandidatesForTest(
		source,
		[]netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")},
	)
	require.NoError(t, err)
	require.Contains(t, candidates, prefix, "installRoutes=false must retain routes in the source BGP RIB")

	target := internalbgp.NewInstance()
	internalbgp.SetInstanceServerForTest(target, targetServer)
	require.NoError(t, target.ReconcileOriginated([]netip.Prefix{originatedPrefix}))
	target.SetOutputState([]netip.Prefix{originatedPrefix}, []resourcenetwork.BGPImportRouteSpec{{
		BGPInstance: "workload",
		Prefixes:    []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")},
	}}, 0, netip.Addr{}, 0, true)

	instances := map[string]*internalbgp.Instance{
		"fabric":   target,
		"workload": source,
	}
	desiredInstances := map[string]struct{}{"fabric": {}, "workload": {}}

	desiredCount, ready, err := internalbgp.ReconcileImportedPaths("fabric", target, instances, desiredInstances)
	require.NoError(t, err)
	require.True(t, ready)
	require.Equal(t, 1, desiredCount, "a prefix originated by the target must not be imported")
	require.True(t, internalbgp.InstanceImportedForTest(target, prefix))
	require.False(t, internalbgp.InstanceImportedForTest(target, originatedPrefix))
	assert.Empty(t, target.Snapshot(ctx).Learned, "an imported path must not become a target RouteSpec")

	echoCandidates, err := internalbgp.ListImportCandidatesForTest(
		target,
		[]netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")},
	)
	require.NoError(t, err)
	assert.Empty(t, echoCandidates, "locally originated and imported paths must not be import candidates")

	imported := listBGPImportTestPath(t, targetServer, prefix)
	require.NotNil(t, imported)
	assert.False(t, imported.PeerAddress.IsValid(), "the imported path must be local to the target server")
	assert.Equal(t, netip.IPv4Unspecified(), internalbgp.PathNexthop(imported))
	assert.Equal(t, uint32(50), importedAttribute[*bgppacket.PathAttributeMultiExitDisc](t, imported).Value)
	assert.Equal(t, []uint32{65000<<16 | 100}, importedAttribute[*bgppacket.PathAttributeCommunities](t, imported).Value)
	require.Eventually(t, func() bool {
		return listBGPImportTestPath(t, fabricServer, prefix) != nil
	}, 5*time.Second, 10*time.Millisecond, "target did not advertise the imported path to its fabric peer")

	updatedPath := newBGPImportTestPath(t, prefix, 75)
	responses, err = sourceServer.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{updatedPath}})
	require.NoError(t, err)
	require.Len(t, responses, 1)
	require.NoError(t, responses[0].Error)

	desiredCount, ready, err = internalbgp.ReconcileImportedPaths("fabric", target, instances, desiredInstances)
	require.NoError(t, err)
	require.True(t, ready)
	require.Equal(t, 1, desiredCount)

	imported = listBGPImportTestPath(t, targetServer, prefix)
	require.NotNil(t, imported)
	assert.Equal(t, uint32(75), importedAttribute[*bgppacket.PathAttributeMultiExitDisc](t, imported).Value)

	internalbgp.SetInstanceServerForTest(source, nil)

	_, ready, err = internalbgp.ReconcileImportedPaths("fabric", target, instances, desiredInstances)
	require.NoError(t, err)
	require.False(t, ready, "a configured but temporarily unavailable source must preserve imports")
	require.True(t, internalbgp.InstanceImportedForTest(target, prefix))

	internalbgp.SetInstanceServerForTest(source, sourceServer)

	delete(instances, "workload")
	desiredCount, ready, err = internalbgp.ReconcileImportedPaths(
		"fabric",
		target,
		instances,
		map[string]struct{}{"fabric": {}},
	)
	require.NoError(t, err)
	require.True(t, ready, "a deliberately removed source must withdraw imports")
	require.Zero(t, desiredCount)
	assert.Zero(t, target.ImportedCount())
	assert.Nil(t, listBGPImportTestPath(t, targetServer, prefix))

	instances["workload"] = source
	desiredCount, ready, err = internalbgp.ReconcileImportedPaths("fabric", target, instances, desiredInstances)
	require.NoError(t, err)
	require.True(t, ready)
	require.Equal(t, 1, desiredCount)
	require.True(t, internalbgp.InstanceImportedForTest(target, prefix))

	require.NoError(t, sourceServer.DeletePath(apiutil.DeletePathRequest{Paths: []*apiutil.Path{updatedPath}}))

	desiredCount, ready, err = internalbgp.ReconcileImportedPaths("fabric", target, instances, desiredInstances)
	require.NoError(t, err)
	require.True(t, ready)
	require.Zero(t, desiredCount)
	assert.Zero(t, target.ImportedCount())
	assert.Nil(t, listBGPImportTestPath(t, targetServer, prefix))
}

func TestBGPImportFamilies(t *testing.T) {
	t.Parallel()

	assert.Equal(t,
		[]bgppacket.Family{bgppacket.RF_IPv4_UC},
		internalbgp.ImportFamiliesForTest([]netip.Prefix{
			netip.MustParsePrefix("198.51.100.0/24"),
			netip.MustParsePrefix("203.0.113.0/24"),
		}),
	)
	assert.Equal(t,
		[]bgppacket.Family{bgppacket.RF_IPv6_UC},
		internalbgp.ImportFamiliesForTest([]netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}),
	)
	assert.Equal(t,
		[]bgppacket.Family{bgppacket.RF_IPv4_UC, bgppacket.RF_IPv6_UC},
		internalbgp.ImportFamiliesForTest([]netip.Prefix{
			netip.MustParsePrefix("198.51.100.0/24"),
			netip.MustParsePrefix("2001:db8::/32"),
		}),
	)
	assert.Empty(t, internalbgp.ImportFamiliesForTest(nil))
}

func startBGPImportTestServer(t *testing.T, ctx context.Context, asn uint32, routerID string, listenPort int32) *gobgpsrv.BgpServer {
	t.Helper()

	server := gobgpsrv.NewBgpServer()
	go server.Serve()

	require.NoError(t, server.StartBgp(ctx, &gobgpapi.StartBgpRequest{Global: &gobgpapi.Global{
		Asn:        asn,
		RouterId:   routerID,
		ListenPort: listenPort,
	}}))

	t.Cleanup(server.Stop)

	return server
}

func connectBGPImportTestFabric(
	t *testing.T,
	ctx context.Context,
	targetServer *gobgpsrv.BgpServer,
	fabricServer *gobgpsrv.BgpServer,
	fabricPort int32,
) {
	t.Helper()

	afiSafis := func() []*gobgpapi.AfiSafi {
		return []*gobgpapi.AfiSafi{
			{Config: &gobgpapi.AfiSafiConfig{Family: &gobgpapi.Family{
				Afi:  gobgpapi.Family_AFI_IP,
				Safi: gobgpapi.Family_SAFI_UNICAST,
			}, Enabled: true}},
			{Config: &gobgpapi.AfiSafiConfig{Family: &gobgpapi.Family{
				Afi:  gobgpapi.Family_AFI_IP6,
				Safi: gobgpapi.Family_SAFI_UNICAST,
			}, Enabled: true}},
		}
	}

	require.NoError(t, fabricServer.AddPeer(ctx, &gobgpapi.AddPeerRequest{Peer: &gobgpapi.Peer{
		Conf: &gobgpapi.PeerConf{
			NeighborAddress: "127.0.0.1",
			PeerAsn:         65001,
		},
		Transport: &gobgpapi.Transport{
			LocalAddress: "127.0.0.2",
			PassiveMode:  true,
		},
		AfiSafis: afiSafis(),
	}}))
	require.NoError(t, targetServer.AddPeer(ctx, &gobgpapi.AddPeerRequest{Peer: &gobgpapi.Peer{
		Conf: &gobgpapi.PeerConf{
			NeighborAddress: "127.0.0.2",
			PeerAsn:         65000,
		},
		Transport: &gobgpapi.Transport{
			LocalAddress: "127.0.0.1",
			RemotePort:   uint32(fabricPort),
		},
		AfiSafis: afiSafis(),
	}}))

	require.Eventually(t, func() bool {
		established := false

		require.NoError(t, targetServer.ListPeer(ctx, &gobgpapi.ListPeerRequest{}, func(peer *gobgpapi.Peer) {
			established = peer.GetState().GetSessionState() == gobgpapi.PeerState_SESSION_STATE_ESTABLISHED
		}))

		return established
	}, 5*time.Second, 10*time.Millisecond, "target and fabric BGP test servers did not establish")
}

func freeBGPImportTestPort(t *testing.T) int32 {
	t.Helper()

	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(t.Context(), "tcp4", "0.0.0.0:0")
	require.NoError(t, err)

	port := listener.Addr().(*net.TCPAddr).Port //nolint:forcetypeassert
	require.NoError(t, listener.Close())

	return int32(port)
}

func newBGPImportTestPath(t *testing.T, prefix netip.Prefix, med uint32) *apiutil.Path {
	t.Helper()

	nlri, err := bgppacket.NewIPAddrPrefix(prefix)
	require.NoError(t, err)

	nexthop, err := bgppacket.NewPathAttributeNextHop(netip.MustParseAddr("192.0.2.10"))
	require.NoError(t, err)

	return &apiutil.Path{
		Family:         bgppacket.RF_IPv4_UC,
		Nlri:           nlri,
		PeerASN:        4200000001,
		PeerID:         netip.MustParseAddr("192.0.2.10"),
		PeerAddress:    netip.MustParseAddr("192.0.2.10"),
		IsFromExternal: true,
		Attrs: []bgppacket.PathAttributeInterface{
			bgppacket.NewPathAttributeOrigin(0),
			bgppacket.NewPathAttributeAsPath([]bgppacket.AsPathParamInterface{
				bgppacket.NewAs4PathParam(2, []uint32{4200000001}),
			}),
			nexthop,
			bgppacket.NewPathAttributeMultiExitDisc(med),
			bgppacket.NewPathAttributeCommunities([]uint32{65000<<16 | 100}),
		},
	}
}

func listBGPImportTestPath(t *testing.T, server *gobgpsrv.BgpServer, prefix netip.Prefix) *apiutil.Path {
	t.Helper()

	var result *apiutil.Path

	require.NoError(t, server.ListPath(apiutil.ListPathRequest{
		TableType: gobgpapi.TableType_TABLE_TYPE_GLOBAL,
		Family:    bgppacket.RF_IPv4_UC,
	}, func(nlri bgppacket.NLRI, paths []*apiutil.Path) {
		if nlri.String() != prefix.String() {
			return
		}

		for _, path := range paths {
			if path.Best && !path.Withdrawal {
				result = path

				return
			}
		}
	}))

	return result
}

func importedAttribute[T bgppacket.PathAttributeInterface](t *testing.T, path *apiutil.Path) T {
	t.Helper()

	for _, attr := range path.Attrs {
		if typed, ok := attr.(T); ok {
			return typed
		}
	}

	var zero T
	t.Fatalf("attribute %T not found", zero)

	return zero
}
