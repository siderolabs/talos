// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/siderolabs/gen/value"
	"github.com/stretchr/testify/assert"

	kubespanadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/kubespan"
	"github.com/siderolabs/talos/internal/app/machined/pkg/adapters/wireguard"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
)

func TestPeerStatus_PickNewEndpoint(t *testing.T) {
	// zero status
	peerStatus := kubespan.PeerStatusSpec{}

	// no endpoint => no way to pick new one
	assert.True(t, value.IsZero(kubespanadapter.PeerStatusSpec(&peerStatus).PickNewEndpoint(nil)))

	endpoints := []netip.AddrPort{
		netip.MustParseAddrPort("10.3.4.5:10500"),
		netip.MustParseAddrPort("192.168.3.8:457"),
	}

	// initial choice should be the first endpoint
	newEndpoint := kubespanadapter.PeerStatusSpec(&peerStatus).PickNewEndpoint(endpoints)
	assert.Equal(t, endpoints[0], newEndpoint)
	kubespanadapter.PeerStatusSpec(&peerStatus).UpdateEndpoint(newEndpoint)

	// next choice should be 2nd endpoint
	newEndpoint = kubespanadapter.PeerStatusSpec(&peerStatus).PickNewEndpoint(endpoints)
	assert.Equal(t, endpoints[1], newEndpoint)
	kubespanadapter.PeerStatusSpec(&peerStatus).UpdateEndpoint(newEndpoint)

	// back to the first endpoint
	newEndpoint = kubespanadapter.PeerStatusSpec(&peerStatus).PickNewEndpoint(endpoints)
	assert.Equal(t, endpoints[0], newEndpoint)
	kubespanadapter.PeerStatusSpec(&peerStatus).UpdateEndpoint(newEndpoint)

	// can't rotate a single endpoint
	assert.True(t, value.IsZero(kubespanadapter.PeerStatusSpec(&peerStatus).PickNewEndpoint(endpoints[:1])))

	// can rotate if the endpoint is different
	newEndpoint = kubespanadapter.PeerStatusSpec(&peerStatus).PickNewEndpoint(endpoints[1:])
	assert.Equal(t, endpoints[1], newEndpoint)
	kubespanadapter.PeerStatusSpec(&peerStatus).UpdateEndpoint(newEndpoint)

	// if totally new list of endpoints is given, pick the first one
	endpoints = []netip.AddrPort{
		netip.MustParseAddrPort("10.3.4.5:10501"),
		netip.MustParseAddrPort("192.168.3.8:458"),
	}
	newEndpoint = kubespanadapter.PeerStatusSpec(&peerStatus).PickNewEndpoint(endpoints)
	assert.Equal(t, endpoints[0], newEndpoint)
	kubespanadapter.PeerStatusSpec(&peerStatus).UpdateEndpoint(newEndpoint)
}

func TestPeerStatus_CalculateState(t *testing.T) {
	for _, tt := range []struct {
		name string

		sinceLastHandshake, sinceEndpointChange time.Duration

		lastUsedEndpointZero bool

		expectedState kubespan.PeerState
	}{
		{
			name:                 "no endpoint set",
			sinceLastHandshake:   time.Hour,
			sinceEndpointChange:  time.Hour,
			lastUsedEndpointZero: true,
			expectedState:        kubespan.PeerStateUnknown,
		},
		{
			name:                "peer is down",
			sinceLastHandshake:  2 * wireguard.PeerDownInterval,
			sinceEndpointChange: 2 * wireguard.PeerDownInterval,
			expectedState:       kubespan.PeerStateDown,
		},
		{
			name:                "fresh peer, no handshake",
			sinceLastHandshake:  2 * wireguard.PeerDownInterval,
			sinceEndpointChange: kubespanadapter.EndpointConnectionTimeout / 2,
			expectedState:       kubespan.PeerStateUnknown,
		},
		{
			name:                "fresh peer, with handshake",
			sinceLastHandshake:  0,
			sinceEndpointChange: kubespanadapter.EndpointConnectionTimeout / 2,
			expectedState:       kubespan.PeerStateUp,
		},
		{
			name:                "peer after initial timeout, with handshake",
			sinceLastHandshake:  0,
			sinceEndpointChange: kubespanadapter.EndpointConnectionTimeout + 1,
			expectedState:       kubespan.PeerStateUp,
		},
		{
			name:                "peer after initial timeout, no handshake",
			sinceLastHandshake:  2 * kubespanadapter.EndpointConnectionTimeout,
			sinceEndpointChange: kubespanadapter.EndpointConnectionTimeout + 1,
			expectedState:       kubespan.PeerStateDown,
		},
		{
			name:                "established peer, up",
			sinceLastHandshake:  wireguard.PeerDownInterval / 2,
			sinceEndpointChange: wireguard.PeerDownInterval + 1,
			expectedState:       kubespan.PeerStateUp,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			peerStatus := kubespan.PeerStatusSpec{
				LastHandshakeTime:  time.Now().Add(-tt.sinceLastHandshake),
				LastEndpointChange: time.Now().Add(-tt.sinceEndpointChange),
			}

			if !tt.lastUsedEndpointZero {
				peerStatus.LastUsedEndpoint = netip.MustParseAddrPort("192.168.1.1:10000")
			}

			kubespanadapter.PeerStatusSpec(&peerStatus).CalculateStateWithDurations(tt.sinceLastHandshake, tt.sinceEndpointChange)

			assert.Equal(t, tt.expectedState, peerStatus.State)
		})
	}
}
