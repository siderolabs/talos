// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"net/netip"
	"time"

	"github.com/siderolabs/gen/value"
	"go4.org/netipx"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/siderolabs/talos/internal/app/machined/pkg/adapters/wireguard"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
)

// PeerStatusSpec adapter provides Wireguard integration and state management.
//
//nolint:revive,golint
func PeerStatusSpec(r *kubespan.PeerStatusSpec) peerStatus {
	return peerStatus{
		PeerStatusSpec: r,
	}
}

type peerStatus struct {
	*kubespan.PeerStatusSpec
}

// EndpointConnectionTimeout is time to wait for initial handshake when the endpoint is just set.
const EndpointConnectionTimeout = 15 * time.Second

// CalculateState updates connection state based on other fields values.
//
// Goal: endpoint is ultimately down if we haven't seen handshake for more than peerDownInterval,
// but as the endpoints get updated we want faster feedback, so we start checking more aggressively
// that the handshake happened within endpointConnectionTimeout since last endpoint change.
//
// Timeline:
//
// ---------------------------------------------------------------------->
// ^            ^                                   ^
// |            |                                   |
// T0           T0+endpointConnectionTimeout        T0+peerDownInterval
//
// Where T0 = LastEndpointChange
//
// The question is where is LastHandshakeTimeout vs. those points above:
//
//   - if we're past (T0+peerDownInterval), simply check that time since last handshake < peerDownInterval
//   - if we're between (T0+endpointConnectionTimeout) and (T0+peerDownInterval), and there's no handshake
//     after the endpoint change, assume that the endpoint is down
//   - if we're between (T0) and (T0+endpointConnectionTimeout), and there's no handshake since the endpoint change,
//     consider the state to be unknown
func (a peerStatus) CalculateState() {
	sinceLastHandshake := time.Since(a.PeerStatusSpec.LastHandshakeTime)
	sinceEndpointChange := time.Since(a.PeerStatusSpec.LastEndpointChange)

	a.CalculateStateWithDurations(sinceLastHandshake, sinceEndpointChange)
}

// CalculateStateWithDurations calculates the state based on the time since events.
func (a peerStatus) CalculateStateWithDurations(sinceLastHandshake, sinceEndpointChange time.Duration) {
	switch {
	case sinceEndpointChange > wireguard.PeerDownInterval: // past T0+peerDownInterval
		// if we got handshake in the last peerDownInterval, endpoint is up
		if sinceLastHandshake < wireguard.PeerDownInterval {
			a.PeerStatusSpec.State = kubespan.PeerStateUp
		} else {
			a.PeerStatusSpec.State = kubespan.PeerStateDown
		}
	case sinceEndpointChange < EndpointConnectionTimeout: // between (T0) and (T0+endpointConnectionTimeout)
		// endpoint got recently updated, consider no handshake as 'unknown'
		if a.PeerStatusSpec.LastHandshakeTime.After(a.PeerStatusSpec.LastEndpointChange) {
			a.PeerStatusSpec.State = kubespan.PeerStateUp
		} else {
			a.PeerStatusSpec.State = kubespan.PeerStateUnknown
		}

	default: // otherwise, we're between (T0+endpointConnectionTimeout) and (T0+peerDownInterval)
		// if we haven't had the handshake yet, consider the endpoint to be down
		if a.PeerStatusSpec.LastHandshakeTime.After(a.PeerStatusSpec.LastEndpointChange) {
			a.PeerStatusSpec.State = kubespan.PeerStateUp
		} else {
			a.PeerStatusSpec.State = kubespan.PeerStateDown
		}
	}

	if a.PeerStatusSpec.State == kubespan.PeerStateDown && value.IsZero(a.PeerStatusSpec.LastUsedEndpoint) {
		// no endpoint, so unknown
		a.PeerStatusSpec.State = kubespan.PeerStateUnknown
	}
}

// UpdateFromWireguard updates fields from wgtypes information.
func (a peerStatus) UpdateFromWireguard(peer wgtypes.Peer) {
	if peer.Endpoint != nil {
		a.PeerStatusSpec.Endpoint, _ = netipx.FromStdAddr(peer.Endpoint.IP, peer.Endpoint.Port, "")
	} else {
		a.PeerStatusSpec.Endpoint = netip.AddrPort{}
	}

	a.PeerStatusSpec.LastHandshakeTime = peer.LastHandshakeTime
	a.PeerStatusSpec.TransmitBytes = peer.TransmitBytes
	a.PeerStatusSpec.ReceiveBytes = peer.ReceiveBytes
}

// UpdateEndpoint updates the endpoint information and last update timestamp.
func (a peerStatus) UpdateEndpoint(endpoint netip.AddrPort) {
	a.PeerStatusSpec.Endpoint = endpoint
	a.PeerStatusSpec.LastUsedEndpoint = endpoint
	a.PeerStatusSpec.LastEndpointChange = time.Now()
	a.PeerStatusSpec.State = kubespan.PeerStateUnknown
}

// ShouldChangeEndpoint tells whether endpoint should be updated.
func (a peerStatus) ShouldChangeEndpoint() bool {
	return a.PeerStatusSpec.State == kubespan.PeerStateDown || value.IsZero(a.PeerStatusSpec.LastUsedEndpoint)
}

// PickNewEndpoint picks new endpoint given the state and list of available endpoints.
//
// If returned newEndpoint is zero value, no new endpoint is available.
func (a peerStatus) PickNewEndpoint(endpoints []netip.AddrPort) (newEndpoint netip.AddrPort) {
	if len(endpoints) == 0 {
		return newEndpoint
	}

	if value.IsZero(a.PeerStatusSpec.LastUsedEndpoint) {
		// first time setting the endpoint
		newEndpoint = endpoints[0]
	} else {
		// find the next endpoint after LastUsedEndpoint and use it
		idx := -1

		for i := range endpoints {
			if endpoints[i] == a.PeerStatusSpec.LastUsedEndpoint {
				idx = i

				break
			}
		}

		// special case: if the peer has just a single endpoint, we can't rotate
		if !(len(endpoints) == 1 && idx == 0 && a.PeerStatusSpec.Endpoint == a.PeerStatusSpec.LastUsedEndpoint) {
			newEndpoint = endpoints[(idx+1)%len(endpoints)]
		}
	}

	return newEndpoint
}
