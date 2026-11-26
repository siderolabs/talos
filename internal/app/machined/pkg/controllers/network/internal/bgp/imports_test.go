// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	resourcenetwork "github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const (
	workloadASN            = 4200000000
	workloadSpeakerASN     = 4200000001
	workloadSpeakerAddress = "127.0.0.2"
	fabricASN              = 65001
	fabricSpeakerASN       = 65000
	fabricSpeakerAddress   = "127.0.0.3"
)

var importSelector = netip.MustParsePrefix("198.51.100.0/24")

// importFixture is a Talos "workload" instance whose routes are imported into a Talos "fabric" instance,
// which advertises them to a fabric speaker.
type importFixture struct {
	workload *testInstance
	fabric   *testInstance

	fabricSpeaker *gobgpsrv.BgpServer

	instances map[string]*internalbgp.Instance
	desired   map[string]struct{}
}

// newImportFixture starts both instances: the workload instance peers with workloadPeer, the fabric speaker is
// connected to the fabric instance.
func newImportFixture(t *testing.T, workloadPeer internalbgp.Peer) *importFixture {
	t.Helper()

	fixture := &importFixture{
		workload: newTestInstance(t),
		fabric:   newTestInstance(t),
	}

	fixture.workload.ensure(t, resourcenetwork.BGPInstanceConfigSpec{LocalASN: workloadASN}, []netip.Addr{netip.MustParseAddr("192.0.2.1")}, workloadPeer)
	fixture.workload.SetOutputState(nil, resourcenetwork.BGPInstanceConfigSpec{InstallRoutes: true})

	fixture.fabric.ensure(t, resourcenetwork.BGPInstanceConfigSpec{LocalASN: fabricASN}, []netip.Addr{netip.MustParseAddr("192.0.2.2")},
		testPeer(fabricSpeakerAddress, fabricSpeakerASN))
	fixture.fabric.SetOutputState(nil, resourcenetwork.BGPInstanceConfigSpec{
		ImportRoutes:  []resourcenetwork.BGPImportRouteSpec{{BGPInstance: "workload", Prefixes: []netip.Prefix{importSelector}}},
		InstallRoutes: true,
	})

	fixture.fabricSpeaker = fixture.fabric.startSpeaker(t, fabricSpeakerAddress, fabricSpeakerASN, fabricASN)
	fixture.fabric.waitState(t, fabricSpeakerAddress, nethelpers.BGPSessionStateEstablished)

	fixture.instances = map[string]*internalbgp.Instance{"fabric": fixture.fabric.Instance, "workload": fixture.workload.Instance}
	fixture.desired = map[string]struct{}{"fabric": {}, "workload": {}}

	return fixture
}

// startWorkloadSpeaker starts a plain workload speaker for the workload instance.
func (fixture *importFixture) startWorkloadSpeaker(t *testing.T) *gobgpsrv.BgpServer {
	t.Helper()

	speaker := fixture.workload.startSpeaker(t, workloadSpeakerAddress, workloadSpeakerASN, workloadASN)
	fixture.workload.waitState(t, workloadSpeakerAddress, nethelpers.BGPSessionStateEstablished)

	return speaker
}

func (fixture *importFixture) reconcile(t *testing.T) (int, bool) {
	t.Helper()

	count, ready, err := internalbgp.ReconcileImportedPaths("fabric", fixture.fabric.Instance, fixture.instances, fixture.desired)
	require.NoError(t, err)

	return count, ready
}

// fabricPath returns the fabric speaker's best path for the prefix, or nil.
func (fixture *importFixture) fabricPath(t *testing.T, prefix netip.Prefix) *apiutil.Path {
	t.Helper()

	return listBGPImportTestPath(t, fixture.fabricSpeaker, prefix)
}

// waitFabricMED waits until the fabric speaker has the prefix with the MED, imported from the workload instance.
func (fixture *importFixture) waitFabricMED(t *testing.T, prefix netip.Prefix, med uint32) {
	t.Helper()

	require.Eventually(t, func() bool {
		fixture.reconcile(t)

		path := fixture.fabricPath(t, prefix)

		if path == nil {
			return false
		}

		attr, ok := findAttribute[*bgppacket.PathAttributeMultiExitDisc](path)

		return ok && attr.Value == med
	}, 15*time.Second, 10*time.Millisecond, "fabric speaker did not get %s with MED %d", prefix, med)
}

func (fixture *importFixture) waitFabricWithdrawn(t *testing.T, prefix netip.Prefix) {
	t.Helper()

	require.Eventually(t, func() bool {
		fixture.reconcile(t)

		return fixture.fabricPath(t, prefix) == nil
	}, 15*time.Second, 10*time.Millisecond, "fabric speaker did not withdraw %s", prefix)
}

// watchFabricWithdrawals counts the withdrawals of the prefix seen by the fabric speaker.
func (fixture *importFixture) watchFabricWithdrawals(t *testing.T, prefix netip.Prefix) *atomic.Int32 {
	t.Helper()

	var withdrawals atomic.Int32

	require.NoError(t, fixture.fabricSpeaker.WatchEvent(t.Context(), gobgpsrv.WatchEventMessageCallbacks{
		OnBestPath: func(paths []*apiutil.Path, _ time.Time) {
			for _, path := range paths {
				if path.Withdrawal && path.Nlri.String() == prefix.String() {
					withdrawals.Add(1)
				}
			}
		},
	}, gobgpsrv.WatchBestPath(false)))

	return &withdrawals
}

// advertise originates the prefix from a speaker with the MED and a community.
func advertise(t *testing.T, speaker *gobgpsrv.BgpServer, prefix netip.Prefix, med uint32) *apiutil.Path {
	t.Helper()

	path, err := internalbgp.BuildOriginatedPath(prefix)
	require.NoError(t, err)

	path.Attrs = append(path.Attrs,
		bgppacket.NewPathAttributeMultiExitDisc(med),
		bgppacket.NewPathAttributeCommunities([]uint32{65000<<16 | 100}),
	)

	responses, err := speaker.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{path}})
	require.NoError(t, err)
	require.Len(t, responses, 1)
	require.NoError(t, responses[0].Error)

	return path
}

func TestBGPRouteImportLifecycle(t *testing.T) {
	t.Parallel()

	fixture := newImportFixture(t, testPeer(workloadSpeakerAddress, workloadSpeakerASN))
	workloadSpeaker := fixture.startWorkloadSpeaker(t)

	prefix := netip.MustParsePrefix("198.51.100.100/32")
	originatedPrefix := netip.MustParsePrefix("198.51.100.101/32")

	advertise(t, workloadSpeaker, prefix, 50)
	advertise(t, workloadSpeaker, originatedPrefix, 60)

	require.Eventually(t, func() bool {
		return len(fixture.workload.Snapshot(t.Context()).Learned) == 2
	}, 5*time.Second, 10*time.Millisecond, "workload instance did not learn the speaker's paths")

	fixture.workload.SetOutputState(nil, resourcenetwork.BGPInstanceConfigSpec{})
	assert.Empty(t, fixture.workload.Snapshot(t.Context()).Learned, "installRoutes=false must suppress FIB output")

	// the fabric instance originates one of the prefixes itself
	require.NoError(t, fixture.fabric.ReconcileOriginated([]netip.Prefix{originatedPrefix}))
	fixture.fabric.SetOutputState([]netip.Prefix{originatedPrefix}, resourcenetwork.BGPInstanceConfigSpec{
		ImportRoutes:  []resourcenetwork.BGPImportRouteSpec{{BGPInstance: "workload", Prefixes: []netip.Prefix{importSelector}}},
		InstallRoutes: true,
	})

	count, ready := fixture.reconcile(t)
	require.True(t, ready)
	require.Equal(t, 1, count, "a prefix originated by the target must not be imported, installRoutes=false must retain routes in the RIB")
	assert.Equal(t, 1, fixture.fabric.ImportedCount())
	assert.Empty(t, fixture.fabric.Snapshot(t.Context()).Learned, "an imported path must not become a target RouteSpec")

	fixture.waitFabricMED(t, prefix, 50)
	assert.Equal(t, []uint32{65000<<16 | 100}, importedAttribute[*bgppacket.PathAttributeCommunities](t, fixture.fabricPath(t, prefix)).Value)

	var originated *apiutil.Path

	require.Eventually(t, func() bool {
		originated = fixture.fabricPath(t, originatedPrefix)

		return originated != nil
	}, 5*time.Second, 10*time.Millisecond, "fabric speaker did not receive the originated path")

	_, imported := findAttribute[*bgppacket.PathAttributeMultiExitDisc](originated)
	assert.False(t, imported, "the fabric must get the originated path, not the imported one")

	advertise(t, workloadSpeaker, prefix, 75)
	fixture.waitFabricMED(t, prefix, 75)

	// a configured but temporarily unavailable source preserves imports
	delete(fixture.instances, "workload")

	_, ready = fixture.reconcile(t)
	require.False(t, ready, "a configured but temporarily unavailable source must preserve imports")
	assert.Equal(t, 1, fixture.fabric.ImportedCount())

	// a deliberately removed source withdraws them
	delete(fixture.desired, "workload")

	count, ready = fixture.reconcile(t)
	require.True(t, ready, "a deliberately removed source must withdraw imports")
	require.Zero(t, count)
	assert.Zero(t, fixture.fabric.ImportedCount())
	fixture.waitFabricWithdrawn(t, prefix)

	fixture.instances["workload"] = fixture.workload.Instance
	fixture.desired["workload"] = struct{}{}

	fixture.waitFabricMED(t, prefix, 75)

	// the source withdraws the path
	path, err := internalbgp.BuildOriginatedPath(prefix)
	require.NoError(t, err)
	require.NoError(t, workloadSpeaker.DeletePath(apiutil.DeletePathRequest{Paths: []*apiutil.Path{path}}))

	fixture.waitFabricWithdrawn(t, prefix)
	assert.Zero(t, fixture.fabric.ImportedCount())
}

func TestImportedPrefixBecomesOriginated(t *testing.T) {
	t.Parallel()

	fixture := newImportFixture(t, testPeer(workloadSpeakerAddress, workloadSpeakerASN))
	workloadSpeaker := fixture.startWorkloadSpeaker(t)

	prefix := netip.MustParsePrefix("198.51.100.100/32")
	other := netip.MustParsePrefix("198.51.100.101/32")

	advertise(t, workloadSpeaker, prefix, 50)
	advertise(t, workloadSpeaker, other, 50)

	fixture.waitFabricMED(t, prefix, 50)
	fixture.waitFabricMED(t, other, 50)

	// an address on an advertised link makes the target originate the imported prefix: the originated path
	// replaces the imported one in GoBGP
	require.NoError(t, fixture.fabric.ReconcileOriginated([]netip.Prefix{prefix}))

	advertise(t, workloadSpeaker, other, 75)
	fixture.waitFabricMED(t, other, 75)

	assert.Equal(t, 1, fixture.fabric.ImportedCount(), "the originated prefix must not be imported")
	require.Eventually(t, func() bool {
		path := fixture.fabricPath(t, prefix)

		if path == nil {
			return false
		}

		_, imported := findAttribute[*bgppacket.PathAttributeMultiExitDisc](path)

		return !imported
	}, 5*time.Second, 10*time.Millisecond, "the fabric must get the originated path, not the imported one")

	// once the target stops originating it, the prefix is imported again
	require.NoError(t, fixture.fabric.ReconcileOriginated(nil))

	fixture.waitFabricMED(t, prefix, 50)
	assert.Equal(t, 2, fixture.fabric.ImportedCount())
}

func TestImportedPathChangeIsReplacedInPlace(t *testing.T) {
	t.Parallel()

	fixture := newImportFixture(t, testPeer(workloadSpeakerAddress, workloadSpeakerASN))
	workloadSpeaker := fixture.startWorkloadSpeaker(t)

	prefix := netip.MustParsePrefix("198.51.100.100/32")

	advertise(t, workloadSpeaker, prefix, 50)
	fixture.waitFabricMED(t, prefix, 50)

	withdrawals := fixture.watchFabricWithdrawals(t, prefix)

	advertise(t, workloadSpeaker, prefix, 75)
	fixture.waitFabricMED(t, prefix, 75)

	assert.Zero(t, withdrawals.Load(), "a changed imported path must replace the old one without a withdrawal")

	// the replacement must still be removable once the source withdraws it
	path, err := internalbgp.BuildOriginatedPath(prefix)
	require.NoError(t, err)
	require.NoError(t, workloadSpeaker.DeletePath(apiutil.DeletePathRequest{Paths: []*apiutil.Path{path}}))

	fixture.waitFabricWithdrawn(t, prefix)
}

func TestImportCandidate(t *testing.T) {
	t.Parallel()

	selected := netip.MustParsePrefix("198.51.100.100/32")
	local := netip.MustParsePrefix("198.51.100.101/32")
	unselected := netip.MustParsePrefix("203.0.113.0/24")
	shorter := netip.MustParsePrefix("198.51.0.0/16")

	lower := newBGPNumberedTestPath(t, selected, netip.MustParseAddr("192.0.2.5"), netip.MustParseAddr("192.0.2.5"))
	higher := newBGPNumberedTestPath(t, selected, netip.MustParseAddr("192.0.2.6"), netip.MustParseAddr("192.0.2.6"))
	notBest := newBGPNumberedTestPath(t, selected, netip.MustParseAddr("192.0.2.4"), netip.MustParseAddr("192.0.2.4"))

	// originated and imported paths are local, without a peer address
	localPath, err := internalbgp.BuildOriginatedPath(local)
	require.NoError(t, err)

	selectors := []netip.Prefix{importSelector, netip.MustParsePrefix("2001:db8::/32")}

	assert.Same(t, lower, internalbgp.ImportCandidate(selected, append(best(higher, lower), notBest), selectors),
		"the best path from the lowest peer address must be selected")
	assert.Nil(t, internalbgp.ImportCandidate(local, best(localPath), selectors), "a local path must not be selected")
	assert.Nil(t, internalbgp.ImportCandidate(unselected, best(newBGPImportTestPath(t, unselected)), selectors),
		"a prefix outside the selectors must not be selected")
	assert.Nil(t, internalbgp.ImportCandidate(shorter, best(newBGPImportTestPath(t, shorter)), selectors),
		"a prefix shorter than its selector must not be selected")
}

func TestBGPImportFamilies(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		[]bgppacket.Family{bgppacket.RF_IPv4_UC},
		internalbgp.ImportFamilies([]netip.Prefix{
			netip.MustParsePrefix("198.51.100.0/24"),
			netip.MustParsePrefix("203.0.113.0/24"),
		}),
	)
	assert.Equal(
		t,
		[]bgppacket.Family{bgppacket.RF_IPv6_UC},
		internalbgp.ImportFamilies([]netip.Prefix{netip.MustParsePrefix("2001:db8::/32")}),
	)
	assert.Equal(
		t,
		[]bgppacket.Family{bgppacket.RF_IPv4_UC, bgppacket.RF_IPv6_UC},
		internalbgp.ImportFamilies([]netip.Prefix{
			netip.MustParsePrefix("198.51.100.0/24"),
			netip.MustParsePrefix("2001:db8::/32"),
		}),
	)
	assert.Empty(t, internalbgp.ImportFamilies(nil))
}

// startBGPTestServer starts a GoBGP server, which does not listen with a negative port.
func startBGPTestServer(t *testing.T, asn uint32, routerID string, listenPort int32) *gobgpsrv.BgpServer {
	t.Helper()

	server := gobgpsrv.NewBgpServer()
	go server.Serve()

	require.NoError(t, server.StartBgp(t.Context(), &gobgpapi.StartBgpRequest{Global: &gobgpapi.Global{
		Asn:        asn,
		RouterId:   routerID,
		ListenPort: listenPort,
	}}))

	t.Cleanup(server.Stop)

	return server
}

// newBGPImportTestPath builds an import candidate path from the numbered peer 192.0.2.10 with MED 50.
func newBGPImportTestPath(t *testing.T, prefix netip.Prefix) *apiutil.Path {
	t.Helper()

	peer := netip.MustParseAddr("192.0.2.10")

	return newBGPNumberedTestPath(
		t, prefix, peer, peer,
		bgppacket.NewPathAttributeMultiExitDisc(50),
		bgppacket.NewPathAttributeCommunities([]uint32{65000<<16 | 100}),
	)
}

// newBGPNumberedTestPath builds an IPv4 path as advertised by a numbered peer, with optional extra attributes.
func newBGPNumberedTestPath(t *testing.T, prefix netip.Prefix, peer, gateway netip.Addr, attrs ...bgppacket.PathAttributeInterface) *apiutil.Path {
	t.Helper()

	nlri, err := bgppacket.NewIPAddrPrefix(prefix)
	require.NoError(t, err)

	nexthop, err := bgppacket.NewPathAttributeNextHop(gateway)
	require.NoError(t, err)

	return &apiutil.Path{
		Family:         bgppacket.RF_IPv4_UC,
		Nlri:           nlri,
		PeerASN:        4200000001,
		PeerID:         peer,
		PeerAddress:    peer,
		IsFromExternal: true,
		Attrs: append([]bgppacket.PathAttributeInterface{
			bgppacket.NewPathAttributeOrigin(0),
			bgppacket.NewPathAttributeAsPath([]bgppacket.AsPathParamInterface{
				bgppacket.NewAs4PathParam(2, []uint32{4200000001}),
			}),
			nexthop,
		}, attrs...),
	}
}

func listBGPImportTestPath(t *testing.T, server *gobgpsrv.BgpServer, prefix netip.Prefix) *apiutil.Path {
	t.Helper()

	var result *apiutil.Path

	family := bgppacket.RF_IPv4_UC
	if prefix.Addr().Is6() {
		family = bgppacket.RF_IPv6_UC
	}

	require.NoError(t, server.ListPath(apiutil.ListPathRequest{
		TableType: gobgpapi.TableType_TABLE_TYPE_GLOBAL,
		Family:    family,
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

func findAttribute[T bgppacket.PathAttributeInterface](path *apiutil.Path) (T, bool) {
	for _, attr := range path.Attrs {
		if typed, ok := attr.(T); ok {
			return typed, true
		}
	}

	var zero T

	return zero, false
}

func importedAttribute[T bgppacket.PathAttributeInterface](t *testing.T, path *apiutil.Path) T {
	t.Helper()

	attr, ok := findAttribute[T](path)
	require.True(t, ok, "attribute not found")

	return attr
}
