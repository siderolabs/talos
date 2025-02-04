// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// EthernetSpecType is type of EthernetSpec resource.
const EthernetSpecType = resource.Type("EthernetSpecs.net.talos.dev")

// EthernetSpec resource holds Ethernet network link status.
type EthernetSpec = typed.Resource[EthernetSpecSpec, EthernetSpecExtension]

// EthernetSpecSpec describes config of Ethernet link.
//
//gotagsrewrite:gen
type EthernetSpecSpec struct {
	Rings EthernetRingsSpec `yaml:"rings,omitempty" protobuf:"1"`
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

	err := protobuf.RegisterDynamic[EthernetSpecSpec](EthernetSpecType, &EthernetSpec{})
	if err != nil {
		panic(err)
	}
}
