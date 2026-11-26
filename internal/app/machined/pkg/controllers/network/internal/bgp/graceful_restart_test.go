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

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const grSpeakerASN = 65010

var grTestPrefix = netip.MustParsePrefix("198.51.100.100/32")

// gracefulRestartFixture is an import fixture whose workload instance peers with a restartable workload speaker
// (like Cilium).
type gracefulRestartFixture struct {
	*importFixture

	fabricWithdrawals *atomic.Int32
}

func newGracefulRestartFixture(t *testing.T, gracefulRestart *network.BGPGracefulRestartConfigSpec) *gracefulRestartFixture {
	t.Helper()

	peer := testPeer(workloadSpeakerAddress, grSpeakerASN)
	peer.Config.GracefulRestart = gracefulRestart

	fixture := &gracefulRestartFixture{importFixture: newImportFixture(t, peer)}
	fixture.fabricWithdrawals = fixture.watchFabricWithdrawals(t, grTestPrefix)

	return fixture
}

// startSpeaker starts a workload speaker advertising grTestPrefix with graceful restart for both unicast families.
//
// A restarted speaker sets the Restart State and Forwarding State bits: without the latter, the helper drops
// the stale paths as soon as the session is re-established instead of waiting for End-of-RIB.
func (fixture *gracefulRestartFixture) startSpeaker(t *testing.T, restartTime uint32, restarted bool) *gobgpsrv.BgpServer {
	t.Helper()

	speaker := startBGPTestServer(t, grSpeakerASN, "192.0.2.10", -1)

	request := fixture.workload.speakerPeer(workloadSpeakerAddress, workloadASN)
	request.Peer.GracefulRestart = &gobgpapi.GracefulRestart{
		Enabled:             true,
		RestartTime:         restartTime,
		NotificationEnabled: true,
		LocalRestarting:     restarted,
		DeferralTime:        30,
	}

	for _, afiSafi := range request.Peer.AfiSafis {
		afiSafi.MpGracefulRestart = &gobgpapi.MpGracefulRestart{Config: &gobgpapi.MpGracefulRestartConfig{Enabled: true}}
	}

	require.NoError(t, speaker.AddPeer(t.Context(), request))

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

	return fixture.workload.peerStatus(t, workloadSpeakerAddress)
}

// learned reports whether the workload instance installs a route for grTestPrefix.
func (fixture *gracefulRestartFixture) learned(t *testing.T) bool {
	t.Helper()

	_, ok := fixture.workload.Snapshot(t.Context()).Learned[grTestPrefix]

	return ok
}

// waitEstablished waits for the speaker's session, and for its path to be learned, imported and advertised to the fabric.
func (fixture *gracefulRestartFixture) waitEstablished(t *testing.T) {
	t.Helper()

	require.Eventually(t, func() bool {
		status := fixture.peerStatus(t)

		return status.State == nethelpers.BGPSessionStateEstablished && !status.PeerRestarting && fixture.learned(t)
	}, 15*time.Second, 10*time.Millisecond, "workload speaker session did not establish")

	require.Eventually(t, func() bool {
		fixture.reconcile(t)

		return fixture.fabricPath(t, grTestPrefix) != nil
	}, 5*time.Second, 10*time.Millisecond, "fabric did not learn the imported path")
}

// assertRetained asserts that the speaker's path survives as stale through the whole pipeline: the installed
// routes, the import into the fabric instance and the advertisement to the fabric peer.
func (fixture *gracefulRestartFixture) assertRetained(t *testing.T) {
	t.Helper()

	assert.True(t, fixture.learned(t), "the stale path must stay installed")

	_, ready := fixture.reconcile(t)
	require.True(t, ready)

	assert.Equal(t, 1, fixture.fabric.ImportedCount(), "the stale path must stay imported")
	assert.NotNil(t, fixture.fabricPath(t, grTestPrefix), "the fabric must keep the imported path")
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

	fixture.waitEstablished(t)

	assert.Equal(t, 1, fixture.fabric.ImportedCount())
	assert.Zero(t, fixture.fabricWithdrawals.Load(), "the fabric must not see a withdrawal through a completed restart")

	// the speaker now advertises a restart time of 3s and never comes back: the stale path is purged
	stopSpeaker(t, speaker)

	require.Eventually(t, func() bool {
		return fixture.peerStatus(t).PeerRestarting
	}, 5*time.Second, 10*time.Millisecond, "the speaker restart was not detected")

	fixture.assertRetained(t)

	require.Eventually(t, func() bool {
		return !fixture.learned(t)
	}, 10*time.Second, 10*time.Millisecond, "the stale path was not purged when the restart time expired")

	assert.False(t, fixture.peerStatus(t).PeerRestarting)

	fixture.waitFabricWithdrawn(t, grTestPrefix)
	assert.Zero(t, fixture.fabric.ImportedCount())
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
		return !fixture.learned(t)
	}, 5*time.Second, 10*time.Millisecond, "the path must be withdrawn at once without graceful restart")

	assert.False(t, fixture.peerStatus(t).PeerRestarting)

	fixture.waitFabricWithdrawn(t, grTestPrefix)
}
