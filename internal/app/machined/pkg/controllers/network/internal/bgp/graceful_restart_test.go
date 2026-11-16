// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"context"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/internal/pkg/testport"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const (
	grSpeakerASN     = 65010
	grSpeakerAddress = "127.0.0.2"
)

// gracefulRestartFixture is a Talos "workload" instance peering with a restartable workload speaker
// (like Cilium), whose routes are imported into a Talos "fabric" instance advertising them to a fabric peer.
type gracefulRestartFixture struct {
	workload     *internalbgp.Instance
	workloadPort int32

	fabric       *internalbgp.Instance
	fabricServer *gobgpsrv.BgpServer

	instances map[string]*internalbgp.Instance
	desired   map[string]struct{}

	fabricWithdrawals atomic.Int32
}

var grTestPrefix = netip.MustParsePrefix("198.51.100.100/32")

func newGracefulRestartFixture(t *testing.T, gracefulRestart *network.BGPGracefulRestartConfigSpec) *gracefulRestartFixture {
	t.Helper()

	ctx := t.Context()

	fixture := &gracefulRestartFixture{
		workload:     internalbgp.NewInstance(),
		workloadPort: int32(testport.Listenable(t)),
		fabric:       internalbgp.NewInstance(),
	}

	t.Cleanup(fixture.workload.Stop)

	// the workload instance goes through EnsureServer, so the peer is built by BuildPeer
	neighbor := network.BGPNeighborConfigSpec{
		Address:         netip.MustParseAddr(grSpeakerAddress),
		PeerASN:         grSpeakerASN,
		Passive:         true,
		GracefulRestart: gracefulRestart,
	}

	require.NoError(t, fixture.workload.EnsureServer(
		ctx,
		zap.NewNop(),
		network.BGPInstanceConfigSpec{LocalASN: 4200000000, Neighbors: []network.BGPNeighborConfigSpec{neighbor}},
		[]netip.Addr{netip.MustParseAddr("192.0.2.1")},
		0,
		[]internalbgp.Peer{{Address: grSpeakerAddress, Config: neighbor}},
		nil,
		fixture.workloadPort,
		func() {},
	))

	fabricPort := int32(testport.Listenable(t))
	targetServer := startBGPImportTestServer(t, ctx, 65001, "192.0.2.2", -1, false)
	fixture.fabricServer = startBGPImportTestServer(t, ctx, 65000, "192.0.2.3", fabricPort, false)
	connectBGPImportTestFabric(t, ctx, targetServer, fixture.fabricServer, fabricPort)

	internalbgp.SetInstanceServerForTest(fixture.fabric, targetServer)
	fixture.fabric.SetOutputState(nil, network.BGPInstanceConfigSpec{
		ImportRoutes: []network.BGPImportRouteSpec{{
			BGPInstance: "workload",
			Prefixes:    []netip.Prefix{netip.MustParsePrefix("198.51.100.0/24")},
		}},
		InstallRoutes: true,
	})

	fixture.workload.SetOutputState(nil, network.BGPInstanceConfigSpec{InstallRoutes: true})

	fixture.instances = map[string]*internalbgp.Instance{"fabric": fixture.fabric, "workload": fixture.workload}
	fixture.desired = map[string]struct{}{"fabric": {}, "workload": {}}

	require.NoError(t, fixture.fabricServer.WatchEvent(ctx, gobgpsrv.WatchEventMessageCallbacks{
		OnBestPath: func(paths []*apiutil.Path, _ time.Time) {
			for _, path := range paths {
				if path.Withdrawal && path.Nlri.String() == grTestPrefix.String() {
					fixture.fabricWithdrawals.Add(1)
				}
			}
		},
	}, gobgpsrv.WatchBestPath(false)))

	return fixture
}

// startSpeaker starts a workload speaker advertising grTestPrefix with graceful restart for both unicast families.
//
// A restarted speaker sets the Restart State and Forwarding State bits: without the latter, the helper drops
// the stale paths as soon as the session is re-established instead of waiting for End-of-RIB.
func (fixture *gracefulRestartFixture) startSpeaker(t *testing.T, restartTime uint32, restarted bool) *gobgpsrv.BgpServer {
	t.Helper()

	ctx := t.Context()

	speaker := startBGPImportTestServer(t, ctx, grSpeakerASN, "192.0.2.10", -1, false)

	afiSafi := func(afi gobgpapi.Family_Afi) *gobgpapi.AfiSafi {
		return &gobgpapi.AfiSafi{
			Config: &gobgpapi.AfiSafiConfig{
				Family:  &gobgpapi.Family{Afi: afi, Safi: gobgpapi.Family_SAFI_UNICAST},
				Enabled: true,
			},
			MpGracefulRestart: &gobgpapi.MpGracefulRestart{Config: &gobgpapi.MpGracefulRestartConfig{Enabled: true}},
		}
	}

	require.NoError(t, speaker.AddPeer(ctx, &gobgpapi.AddPeerRequest{Peer: &gobgpapi.Peer{
		Conf: &gobgpapi.PeerConf{NeighborAddress: "127.0.0.1", PeerAsn: 4200000000},
		Transport: &gobgpapi.Transport{
			LocalAddress: grSpeakerAddress,
			RemotePort:   uint32(fixture.workloadPort),
		},
		GracefulRestart: &gobgpapi.GracefulRestart{
			Enabled:             true,
			RestartTime:         restartTime,
			NotificationEnabled: true,
			LocalRestarting:     restarted,
			DeferralTime:        30,
		},
		AfiSafis: []*gobgpapi.AfiSafi{afiSafi(gobgpapi.Family_AFI_IP), afiSafi(gobgpapi.Family_AFI_IP6)},
	}}))

	path, err := internalbgp.BuildOriginatedPath(grTestPrefix)
	require.NoError(t, err)

	_, err = speaker.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{path}})
	require.NoError(t, err)

	return speaker
}

// stopSpeaker stops the speaker the way a restarting GoBGP does: the TCP connection is closed without a NOTIFICATION.
func stopSpeaker(t *testing.T, speaker *gobgpsrv.BgpServer) {
	t.Helper()

	require.NoError(t, speaker.StopBgp(context.Background(), &gobgpapi.StopBgpRequest{AllowGracefulRestart: true}))
}

func (fixture *gracefulRestartFixture) peerStatus(t *testing.T) network.BGPPeerStatusSpec {
	t.Helper()

	statuses := fixture.workload.Snapshot(t.Context()).PeerStatuses
	require.Len(t, statuses, 1)

	return statuses[0]
}

// learnedPath returns the workload instance's best path for grTestPrefix, or nil.
func (fixture *gracefulRestartFixture) learnedPath(t *testing.T) *apiutil.Path {
	t.Helper()

	return listBGPImportTestPath(t, internalbgp.InstanceServerForTest(fixture.workload), grTestPrefix)
}

func (fixture *gracefulRestartFixture) reconcileImports(t *testing.T) {
	t.Helper()

	_, ready, err := internalbgp.ReconcileImportedPaths("fabric", fixture.fabric, fixture.instances, fixture.desired)
	require.NoError(t, err)
	require.True(t, ready)
}

// waitEstablished waits for the speaker's session, and for its path to be learned, imported and advertised to the fabric.
func (fixture *gracefulRestartFixture) waitEstablished(t *testing.T) {
	t.Helper()

	require.Eventually(t, func() bool {
		path := fixture.learnedPath(t)

		return fixture.peerStatus(t).State == nethelpers.BGPSessionStateEstablished && path != nil && !path.Stale
	}, 15*time.Second, 10*time.Millisecond, "workload speaker session did not establish")

	fixture.reconcileImports(t)

	require.Eventually(t, func() bool {
		return listBGPImportTestPath(t, fixture.fabricServer, grTestPrefix) != nil
	}, 5*time.Second, 10*time.Millisecond, "fabric did not learn the imported path")
}

// assertRetained asserts that the speaker's path survives as stale through the whole pipeline: the workload RIB,
// the installed routes, the import into the fabric instance and the advertisement to the fabric peer.
func (fixture *gracefulRestartFixture) assertRetained(t *testing.T) {
	t.Helper()

	path := fixture.learnedPath(t)
	require.NotNil(t, path, "the stale path must remain the best path")
	assert.True(t, path.Stale)
	assert.Contains(t, fixture.workload.Snapshot(t.Context()).Learned, grTestPrefix, "the stale path must stay installed")

	fixture.reconcileImports(t)

	assert.True(t, internalbgp.InstanceImportedForTest(fixture.fabric, grTestPrefix), "the stale path must stay imported")
	assert.NotNil(t, listBGPImportTestPath(t, fixture.fabricServer, grTestPrefix), "the fabric must keep the imported path")
	assert.Zero(t, fixture.fabricWithdrawals.Load(), "the fabric must not see a withdrawal")
}

func TestGracefulRestartHelper(t *testing.T) {
	t.Parallel()

	fixture := newGracefulRestartFixture(t, &network.BGPGracefulRestartConfigSpec{Notification: true})

	speaker := fixture.startSpeaker(t, 120, false)
	fixture.waitEstablished(t)

	status := fixture.peerStatus(t)
	assert.True(t, status.GracefulRestart, "graceful restart must be negotiated")
	assert.False(t, status.PeerRestarting)

	stopSpeaker(t, speaker)

	require.Eventually(t, func() bool {
		return fixture.peerStatus(t).PeerRestarting
	}, 5*time.Second, 10*time.Millisecond, "the speaker restart was not detected")

	assert.NotEqual(t, nethelpers.BGPSessionStateEstablished, fixture.peerStatus(t).State)
	fixture.assertRetained(t)

	// the restarted speaker re-advertises the path and sends End-of-RIB, which ends the restart
	speaker = fixture.startSpeaker(t, 3, true)

	require.Eventually(t, func() bool {
		path := fixture.learnedPath(t)
		status := fixture.peerStatus(t)

		return status.State == nethelpers.BGPSessionStateEstablished && !status.PeerRestarting && path != nil && !path.Stale
	}, 15*time.Second, 10*time.Millisecond, "the restart did not complete")

	fixture.reconcileImports(t)

	assert.True(t, internalbgp.InstanceImportedForTest(fixture.fabric, grTestPrefix))
	assert.NotNil(t, listBGPImportTestPath(t, fixture.fabricServer, grTestPrefix))
	assert.Zero(t, fixture.fabricWithdrawals.Load(), "the fabric must not see a withdrawal through a completed restart")

	// the speaker now advertises a restart time of 3s and never comes back: the stale path is purged
	stopSpeaker(t, speaker)

	require.Eventually(t, func() bool {
		return fixture.peerStatus(t).PeerRestarting
	}, 5*time.Second, 10*time.Millisecond, "the speaker restart was not detected")

	fixture.assertRetained(t)

	require.Eventually(t, func() bool {
		return fixture.learnedPath(t) == nil
	}, 10*time.Second, 10*time.Millisecond, "the stale path was not purged when the restart time expired")

	assert.False(t, fixture.peerStatus(t).PeerRestarting)
	assert.NotContains(t, fixture.workload.Snapshot(t.Context()).Learned, grTestPrefix)

	fixture.reconcileImports(t)

	assert.False(t, internalbgp.InstanceImportedForTest(fixture.fabric, grTestPrefix))
	require.Eventually(t, func() bool {
		return listBGPImportTestPath(t, fixture.fabricServer, grTestPrefix) == nil
	}, 5*time.Second, 10*time.Millisecond, "the purge did not propagate to the fabric")
}

func TestGracefulRestartDisabled(t *testing.T) {
	t.Parallel()

	fixture := newGracefulRestartFixture(t, nil)

	speaker := fixture.startSpeaker(t, 120, false)
	fixture.waitEstablished(t)

	assert.False(t, fixture.peerStatus(t).GracefulRestart, "graceful restart must not be negotiated unless enabled")

	stopSpeaker(t, speaker)

	// well within the restart time advertised by the speaker
	require.Eventually(t, func() bool {
		return fixture.learnedPath(t) == nil
	}, 5*time.Second, 10*time.Millisecond, "the path must be withdrawn at once without graceful restart")

	assert.False(t, fixture.peerStatus(t).PeerRestarting)

	fixture.reconcileImports(t)

	require.Eventually(t, func() bool {
		return listBGPImportTestPath(t, fixture.fabricServer, grTestPrefix) == nil
	}, 5*time.Second, 10*time.Millisecond, "the withdrawal did not propagate to the fabric")
}
