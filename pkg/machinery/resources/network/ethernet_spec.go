// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"maps"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// EthernetSpecType is type of EthernetSpec resource.
const EthernetSpecType = resource.Type("EthernetSpecs.net.talos.dev")

// EthernetSpec resource holds Ethernet network link status.
type EthernetSpec = typed.Resource[EthernetSpecSpec, EthernetSpecExtension]

// EthernetSpecSpec describes config of Ethernet link.
//
//gotagsrewrite:gen
//redactgen:gen
type EthernetSpecSpec struct {
	Rings        EthernetRingsSpec       `yaml:"rings,omitempty" protobuf:"1"`
	Features     map[string]bool         `yaml:"features,omitempty" protobuf:"2"`
	Channels     EthernetChannelsSpec    `yaml:"channels,omitempty" protobuf:"3"`
	WakeOnLAN    []nethelpers.WOLMode    `yaml:"wakeOnLan,omitempty" protobuf:"4"`
	FlowControl  EthernetFlowControlSpec `yaml:"flowControl,omitempty" protobuf:"5"`
	LinkName     string                  `yaml:"linkName,omitempty" protobuf:"6"`
	FromSelector bool                    `yaml:"fromSelector,omitempty" protobuf:"7"`
}

// EthernetRingsSpec describes config of Ethernet rings.
//
//gotagsrewrite:gen
type EthernetRingsSpec struct {
	RX           *uint32 `yaml:"rx,omitempty" protobuf:"1"`
	TX           *uint32 `yaml:"tx,omitempty" protobuf:"4"`
	RXMini       *uint32 `yaml:"rx-mini,omitempty" protobuf:"2"`
	RXJumbo      *uint32 `yaml:"rx-jumbo,omitempty" protobuf:"3"`
	RXBufLen     *uint32 `yaml:"rx-buf-len,omitempty" protobuf:"5"`
	CQESize      *uint32 `yaml:"cqe-size,omitempty" protobuf:"6"`
	TXPush       *bool   `yaml:"tx-push,omitempty" protobuf:"7"`
	RXPush       *bool   `yaml:"rx-push,omitempty" protobuf:"8"`
	TXPushBufLen *uint32 `yaml:"tx-push-buf-len,omitempty" protobuf:"9"`
	TCPDataSplit *bool   `yaml:"tcp-data-split,omitempty" protobuf:"10"`
}

// EthernetFlowControlSpec describes config of Ethernet flow control.
//
//gotagsrewrite:gen
type EthernetFlowControlSpec struct {
	RX      *bool `yaml:"rx,omitempty" protobuf:"1"`
	TX      *bool `yaml:"tx,omitempty" protobuf:"2"`
	Autoneg *bool `yaml:"autoneg,omitempty" protobuf:"3"`
}

// EthernetChannelsSpec describes config of Ethernet channels.
//
//gotagsrewrite:gen
type EthernetChannelsSpec struct {
	RX       *uint32 `yaml:"rx,omitempty" protobuf:"1"`
	TX       *uint32 `yaml:"tx,omitempty" protobuf:"2"`
	Other    *uint32 `yaml:"other,omitempty" protobuf:"3"`
	Combined *uint32 `yaml:"combined,omitempty" protobuf:"4"`
}

// Merge with other, overwriting fields from other if set.
func (spec *EthernetSpecSpec) Merge(other *EthernetSpecSpec) {
	updateIfNotNil(&spec.Rings.RX, other.Rings.RX)
	updateIfNotNil(&spec.Rings.TX, other.Rings.TX)
	updateIfNotNil(&spec.Rings.RXMini, other.Rings.RXMini)
	updateIfNotNil(&spec.Rings.RXJumbo, other.Rings.RXJumbo)
	updateIfNotNil(&spec.Rings.RXBufLen, other.Rings.RXBufLen)
	updateIfNotNil(&spec.Rings.CQESize, other.Rings.CQESize)
	updateIfNotNil(&spec.Rings.TXPush, other.Rings.TXPush)
	updateIfNotNil(&spec.Rings.RXPush, other.Rings.RXPush)
	updateIfNotNil(&spec.Rings.TXPushBufLen, other.Rings.TXPushBufLen)
	updateIfNotNil(&spec.Rings.TCPDataSplit, other.Rings.TCPDataSplit)

	updateIfNotNil(&spec.Channels.RX, other.Channels.RX)
	updateIfNotNil(&spec.Channels.TX, other.Channels.TX)
	updateIfNotNil(&spec.Channels.Other, other.Channels.Other)
	updateIfNotNil(&spec.Channels.Combined, other.Channels.Combined)

	updateIfNotNil(&spec.FlowControl.RX, other.FlowControl.RX)
	updateIfNotNil(&spec.FlowControl.TX, other.FlowControl.TX)
	updateIfNotNil(&spec.FlowControl.Autoneg, other.FlowControl.Autoneg)

	if other.Features != nil {
		if spec.Features == nil {
			spec.Features = make(map[string]bool, len(other.Features))
		}

		maps.Copy(spec.Features, other.Features)
	}

	if other.WakeOnLAN != nil {
		spec.WakeOnLAN = slices.Clone(other.WakeOnLAN)
	}
}

func updateIfNotNil[T any](target **T, source *T) {
	if source != nil {
		*target = new(*source)
	}
}

// NewEthernetSpec initializes a EthernetSpec resource.
func NewEthernetSpec(namespace resource.Namespace, id resource.ID) *EthernetSpec {
	return typed.NewResource[EthernetSpecSpec, EthernetSpecExtension](
		resource.NewMetadata(namespace, EthernetSpecType, id, resource.VersionUndefined),
		EthernetSpecSpec{},
	)
}

// EthernetSpecExtension provides auxiliary methods for EthernetSpec.
type EthernetSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (EthernetSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EthernetSpecType,
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.NonSensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(EthernetSpecType, &EthernetSpec{})
	if err != nil {
		panic(err)
	}
}
