// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// EthernetStatusType is type of EthernetStatus resource.
const EthernetStatusType = resource.Type("EthernetStatuses.net.talos.dev")

// EthernetStatus resource holds Ethernet network link status.
type EthernetStatus = typed.Resource[EthernetStatusSpec, EthernetStatusExtension]

// EthernetStatusSpec describes status of rendered secrets.
//
//gotagsrewrite:gen
type EthernetStatusSpec struct {
	LinkState     *bool                `yaml:"linkState,omitempty" protobuf:"1"`
	SpeedMegabits int                  `yaml:"speedMbit,omitempty" protobuf:"2"`
	Port          nethelpers.Port      `yaml:"port" protobuf:"3"`
	Duplex        nethelpers.Duplex    `yaml:"duplex" protobuf:"4"`
	OurModes      []string             `yaml:"ourModes,omitempty" protobuf:"5"`
	PeerModes     []string             `yaml:"peerModes,omitempty" protobuf:"6"`
	Rings         *EthernetRingsStatus `yaml:"rings,omitempty" protobuf:"7"`
}

// EthernetRingsStatus describes status of Ethernet rings.
//
//gotagsrewrite:gen
type EthernetRingsStatus struct {
	// Read-only settings.
	RXMax           *uint32 `yaml:"rx-max,omitempty" protobuf:"1"`
	RXMiniMax       *uint32 `yaml:"rx-mini-max,omitempty" protobuf:"2"`
	RXJumboMax      *uint32 `yaml:"rx-jumbo-max,omitempty" protobuf:"3"`
	TXMax           *uint32 `yaml:"tx-max,omitempty" protobuf:"4"`
	TXPushBufLenMax *uint32 `yaml:"tx-push-buf-len-max,omitempty" protobuf:"5"`

	// Current settings (read-write).
	RX           *uint32 `yaml:"rx,omitempty" protobuf:"6"`
	RXMini       *uint32 `yaml:"rx-mini,omitempty" protobuf:"7"`
	RXJumbo      *uint32 `yaml:"rx-jumbo,omitempty" protobuf:"8"`
	TX           *uint32 `yaml:"tx,omitempty" protobuf:"9"`
	RXBufLen     *uint32 `yaml:"rx-buf-len,omitempty" protobuf:"10"`
	CQESize      *uint32 `yaml:"cqe-size,omitempty" protobuf:"11"`
	TXPush       *bool   `yaml:"tx-push,omitempty" protobuf:"12"`
	RXPush       *bool   `yaml:"rx-push,omitempty" protobuf:"13"`
	TXPushBufLen *uint32 `yaml:"tx-push-buf-len,omitempty" protobuf:"14"`
	TCPDataSplit *bool   `yaml:"tcp-data-split,omitempty" protobuf:"15"`
}

// NewEthernetStatus initializes a EthernetStatus resource.
func NewEthernetStatus(namespace resource.Namespace, id resource.ID) *EthernetStatus {
	return typed.NewResource[EthernetStatusSpec, EthernetStatusExtension](
		resource.NewMetadata(namespace, EthernetStatusType, id, resource.VersionUndefined),
		EthernetStatusSpec{},
	)
}

// EthernetStatusExtension provides auxiliary methods for EthernetStatus.
type EthernetStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EthernetStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EthernetStatusType,
		Aliases:          []resource.Type{"ethtool"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Link",
				JSONPath: `{.linkState}`,
			},
			{
				Name:     "Speed",
				JSONPath: `{.speedMbit}`,
			},
		},
		Sensitivity: meta.NonSensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[EthernetStatusSpec](EthernetStatusType, &EthernetStatus{})
	if err != nil {
		panic(err)
	}
}
