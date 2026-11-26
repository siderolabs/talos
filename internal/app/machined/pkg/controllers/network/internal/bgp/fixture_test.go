// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"net/netip"
	"testing"
	"time"

	gobgpapi "github.com/osrg/gobgp/v4/api"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	internalbgp "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/bgp"
	"github.com/siderolabs/talos/internal/pkg/testport"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// testInstance is an instance under test, listening for its peers on a loopback port.
type testInstance struct {
	*internalbgp.Instance

	port       int32
	logger     *zap.Logger
	logs       *observer.ObservedLogs
	peerIfaces map[netip.Addr]string
}

func newTestInstance(t *testing.T) *testInstance {
	t.Helper()

	core, logs := observer.New(zap.DebugLevel)

	instance := &testInstance{
		Instance: internalbgp.NewInstance(),
		port:     int32(testport.Listenable(t)),
		logger:   zap.New(core),
		logs:     logs,
	}

	t.Cleanup(instance.Stop)

	return instance
}

// testPeer is a passive neighbor, for a speaker connecting from a loopback address.
func testPeer(address string, asn uint32) internalbgp.Peer {
	return internalbgp.Peer{
		Address: address,
		Config:  network.BGPNeighborConfigSpec{Address: netip.MustParseAddr(address), PeerASN: asn, Passive: true},
	}
}

// ensure reconciles the instance to the config and peers, with the router ID derived from the candidates.
func (instance *testInstance) ensure(t *testing.T, config network.BGPInstanceConfigSpec, candidates []netip.Addr, peers ...internalbgp.Peer) {
	t.Helper()

	config.Neighbors = nil

	for _, peer := range peers {
		config.Neighbors = append(config.Neighbors, peer.Config)
	}

	require.NoError(t, instance.EnsureServer(t.Context(), instance.logger, config, candidates, 0, peers, instance.peerIfaces, instance.port, func() {}))
}

// starts returns how many times the GoBGP server was started.
func (instance *testInstance) starts() int {
	return instance.logs.FilterMessage("started embedded BGP speaker").Len()
}

// routerID returns the router ID the GoBGP server was last started with.
func (instance *testInstance) routerID(t *testing.T) string {
	t.Helper()

	started := instance.logs.FilterMessage("started embedded BGP speaker").All()
	require.NotEmpty(t, started)

	return started[len(started)-1].ContextMap()["router_id"].(string) //nolint:forcetypeassert
}

// peerStatus returns the status of the peer at address.
func (instance *testInstance) peerStatus(t *testing.T, address string) network.BGPPeerStatusSpec {
	t.Helper()

	for _, status := range instance.Snapshot(t.Context()).PeerStatuses {
		if status.Peer == address {
			return status
		}
	}

	require.FailNow(t, "peer is not configured", address)

	return network.BGPPeerStatusSpec{}
}

func (instance *testInstance) waitState(t *testing.T, address string, state nethelpers.BGPSessionState) {
	t.Helper()

	require.Eventually(t, func() bool {
		return instance.peerStatus(t, address).State == state
	}, 15*time.Second, 10*time.Millisecond, "BGP session with %s did not reach %s", address, state)
}

// startSpeaker starts a GoBGP speaker which connects from address to the instance, for both unicast families.
func (instance *testInstance) startSpeaker(t *testing.T, address string, asn, instanceASN uint32) *gobgpsrv.BgpServer {
	t.Helper()

	speaker := startBGPTestServer(t, asn, address, -1)

	require.NoError(t, speaker.AddPeer(t.Context(), instance.speakerPeer(address, instanceASN)))

	return speaker
}

// speakerPeer is the instance as a neighbor of a speaker connecting from address.
func (instance *testInstance) speakerPeer(address string, instanceASN uint32) *gobgpapi.AddPeerRequest {
	return &gobgpapi.AddPeerRequest{Peer: &gobgpapi.Peer{
		Conf:      &gobgpapi.PeerConf{NeighborAddress: "127.0.0.1", PeerAsn: instanceASN},
		Transport: &gobgpapi.Transport{LocalAddress: address, RemotePort: uint32(instance.port)},
		// the instance closes connections accepted before its peer leaves Idle: retry them quickly
		Timers:   &gobgpapi.Timers{Config: &gobgpapi.TimersConfig{ConnectRetry: 1}},
		AfiSafis: []*gobgpapi.AfiSafi{unicast(gobgpapi.Family_AFI_IP), unicast(gobgpapi.Family_AFI_IP6)},
	}}
}

func unicast(afi gobgpapi.Family_Afi) *gobgpapi.AfiSafi {
	return &gobgpapi.AfiSafi{
		Config: &gobgpapi.AfiSafiConfig{Family: &gobgpapi.Family{Afi: afi, Safi: gobgpapi.Family_SAFI_UNICAST}, Enabled: true},
	}
}
