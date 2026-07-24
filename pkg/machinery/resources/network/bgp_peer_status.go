// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"net/netip"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// BGPPeerStatusType is type of BGPPeerStatus resource.
const BGPPeerStatusType = resource.Type("BGPPeerStatuses.net.talos.dev")

// BGPPeerStatus resource holds the observed state of a BGP peering session.
type BGPPeerStatus = typed.Resource[BGPPeerStatusSpec, BGPPeerStatusExtension]

// BGPPeerStatusSpec describes the status of a BGP peering session.
//
//gotagsrewrite:gen
type BGPPeerStatusSpec struct {
	Peer       string                     `yaml:"peer" protobuf:"1"`
	LocalASN   uint32                     `yaml:"localASN" protobuf:"2"`
	PeerASN    uint32                     `yaml:"peerASN" protobuf:"3"`
	State      nethelpers.BGPSessionState `yaml:"state" protobuf:"4"`
	RouterID   netip.Addr                 `yaml:"routerID,omitempty" protobuf:"5"`
	Since      time.Time                  `yaml:"since,omitempty" protobuf:"6"`
	Received   uint32                     `yaml:"received" protobuf:"7"`
	Advertised uint32                     `yaml:"advertised" protobuf:"8"`
	Accepted   uint32                     `yaml:"accepted" protobuf:"9"`
	BFDState   string                     `yaml:"bfdState,omitempty" protobuf:"10"`
	Instance   string                     `yaml:"instance" protobuf:"11"`
}

// NewBGPPeerStatus initializes a BGPPeerStatus resource.
func NewBGPPeerStatus(namespace resource.Namespace, id resource.ID) *BGPPeerStatus {
	return typed.NewResource[BGPPeerStatusSpec, BGPPeerStatusExtension](
		resource.NewMetadata(namespace, BGPPeerStatusType, id, resource.VersionUndefined),
		BGPPeerStatusSpec{},
	)
}

// BGPPeerStatusExtension provides auxiliary methods for BGPPeerStatus.
type BGPPeerStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (BGPPeerStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BGPPeerStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Instance",
				JSONPath: `{.instance}`,
			},
			{
				Name:     "Peer",
				JSONPath: `{.peer}`,
			},
			{
				Name:     "Peer AS",
				JSONPath: `{.peerASN}`,
			},
			{
				Name:     "State",
				JSONPath: `{.state}`,
			},
			{
				Name:     "Since",
				JSONPath: `{.since}`,
			},
			{
				Name:     "Received",
				JSONPath: `{.received}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[BGPPeerStatusSpec](BGPPeerStatusType, &BGPPeerStatus{})
	if err != nil {
		panic(err)
	}
}
