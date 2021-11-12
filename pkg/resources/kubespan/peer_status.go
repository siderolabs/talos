// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
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
