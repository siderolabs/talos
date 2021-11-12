// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"inet.af/netaddr"
)

// PeerStatusType is type of PeerStatus resource.
const PeerStatusType = resource.Type("KubeSpanPeerStatuses.kubespan.talos.dev")

// PeerStatus the Wireguard peer state for KubeSpan.
//
// PeerStatus is identified by the public key.
type PeerStatus struct {
	md   resource.Metadata
	spec PeerStatusSpec
}

// PeerStatusSpec describes PeerStatus state.
type PeerStatusSpec struct {
	// Active endpoint as seen by the Wireguard.
	Endpoint netaddr.IPPort `yaml:"endpoint"`
	// Label derived from the peer spec.
	Label string `yaml:"label"`
	// Calculated state.
	State PeerState `yaml:"state"`
	// Tx/Rx bytes.
	ReceiveBytes  int64 `yaml:"receiveBytes"`
	TransmitBytes int64 `yaml:"transmitBytes"`
	// Handshake.
	LastHandshakeTime time.Time `yaml:"lastHandshakeTime"`
	// Endpoint selection input.
	LastUsedEndpoint   netaddr.IPPort `yaml:"lastUsedEndpoint"`
	LastEndpointChange time.Time      `yaml:"lastEndpointChange"`
}

// NewPeerStatus initializes a PeerStatus resource.
func NewPeerStatus(namespace resource.Namespace, id resource.ID) *PeerStatus {
	r := &PeerStatus{
		md:   resource.NewMetadata(namespace, PeerStatusType, id, resource.VersionUndefined),
		spec: PeerStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *PeerStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *PeerStatus) Spec() interface{} {
	return r.spec
}

func (r *PeerStatus) String() string {
	return fmt.Sprintf("kubespan.PeerStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *PeerStatus) DeepCopy() resource.Resource {
	return &PeerStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *PeerStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             PeerStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Label",
				JSONPath: `{.label}`,
			},
			{
				Name:     "Endpoint",
				JSONPath: `{.endpoint}`,
			},
			{
				Name:     "State",
				JSONPath: `{.state}`,
			},
			{
				Name:     "Rx",
				JSONPath: `{.receiveBytes}`,
			},
			{
				Name:     "Tx",
				JSONPath: `{.transmitBytes}`,
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *PeerStatus) TypedSpec() *PeerStatusSpec {
	return &r.spec
}

// PeerDownInterval is the time since last handshake when established peer is considered to be down.
//
// WG whitepaper defines a downed peer as being:
// Handshake Timeout (180s) + Rekey Timeout (5s) + Rekey Attempt Timeout (90s)
//
// This interval is applied when the link is already established.
const PeerDownInterval = (180 + 5 + 90) * time.Second

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
// Where T0 = LastEndpontChange
//
// The question is where is LastHandshakeTimeout vs. those points above:
//
//   * if we're past (T0+peerDownInterval), simply check that time since last handshake < peerDownInterval
//   * if we're between (T0+endpointConnectionTimeout) and (T0+peerDownInterval), and there's no handshake
//     after the endpoint change, assume that the endpoint is down
//   * if we're between (T0) and (T0+endpointConnectionTimeout), and there's no handshake since the endpoint change,
//     consider the state to be unknown
func (spec *PeerStatusSpec) CalculateState() {
	sinceLastHandshake := time.Since(spec.LastHandshakeTime)
	sinceEndpointChange := time.Since(spec.LastEndpointChange)

	spec.CalculateStateWithDurations(sinceLastHandshake, sinceEndpointChange)
}

// CalculateStateWithDurations calculates the state based on the time since events.
func (spec *PeerStatusSpec) CalculateStateWithDurations(sinceLastHandshake, sinceEndpointChange time.Duration) {
	switch {
	case sinceEndpointChange > PeerDownInterval: // past T0+peerDownInterval
		// if we got handshake in the last peerDownInterval, endpoint is up
		if sinceLastHandshake < PeerDownInterval {
			spec.State = PeerStateUp
		} else {
			spec.State = PeerStateDown
		}
	case sinceEndpointChange < EndpointConnectionTimeout: // between (T0) and (T0+endpointConnectionTimeout)
		// endpoint got recently updated, consider no handshake as 'unknown'
		if spec.LastHandshakeTime.After(spec.LastEndpointChange) {
			spec.State = PeerStateUp
		} else {
			spec.State = PeerStateUnknown
		}

	default: // otherwise, we're between (T0+endpointConnectionTimeout) and (T0+peerDownInterval)
		// if we haven't had the handshake yet, consider the endpoint to be down
		if spec.LastHandshakeTime.After(spec.LastEndpointChange) {
			spec.State = PeerStateUp
		} else {
			spec.State = PeerStateDown
		}
	}

	if spec.State == PeerStateDown && spec.LastUsedEndpoint.IsZero() {
		// no endpoint, so unknown
		spec.State = PeerStateUnknown
	}
}

// UpdateFromWireguard updates fields from wgtypes information.
func (spec *PeerStatusSpec) UpdateFromWireguard(peer wgtypes.Peer) {
	if peer.Endpoint != nil {
		spec.Endpoint, _ = netaddr.FromStdAddr(peer.Endpoint.IP, peer.Endpoint.Port, "")
	} else {
		spec.Endpoint = netaddr.IPPort{}
	}

	spec.LastHandshakeTime = peer.LastHandshakeTime
	spec.TransmitBytes = peer.TransmitBytes
	spec.ReceiveBytes = peer.ReceiveBytes
}

// UpdateEndpoint updates the endpoint information and last update timestamp.
func (spec *PeerStatusSpec) UpdateEndpoint(endpoint netaddr.IPPort) {
	spec.Endpoint = endpoint
	spec.LastUsedEndpoint = endpoint
	spec.LastEndpointChange = time.Now()
	spec.State = PeerStateUnknown
}

// ShouldChangeEndpoint tells whether endpoint should be updated.
func (spec *PeerStatusSpec) ShouldChangeEndpoint() bool {
	return spec.State == PeerStateDown || spec.LastUsedEndpoint.IsZero()
}

// PickNewEndpoint picks new endpoint given the state and list of available endpoints.
//
// If returned newEndpoint is zero value, no new endpoint is available.
func (spec *PeerStatusSpec) PickNewEndpoint(endpoints []netaddr.IPPort) (newEndpoint netaddr.IPPort) {
	if len(endpoints) == 0 {
		return
	}

	if spec.LastUsedEndpoint.IsZero() {
		// first time setting the endpoint
		newEndpoint = endpoints[0]
	} else {
		// find the next endpoint after LastUsedEndpoint and use it
		idx := -1

		for i := range endpoints {
			if endpoints[i] == spec.LastUsedEndpoint {
				idx = i

				break
			}
		}

		// special case: if the peer has just a single endpoint, we can't rotate
		if !(len(endpoints) == 1 && idx == 0 && spec.Endpoint == spec.LastUsedEndpoint) {
			newEndpoint = endpoints[(idx+1)%len(endpoints)]
		}
	}

	return
}
