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

// LLDPNeighborStatusType is the type of LLDPNeighborStatus resource.
const LLDPNeighborStatusType = resource.Type("LLDPNeighborStatuses.net.talos.dev")

// LLDPNeighborStatus contains all LLDP neighbors observed on one link.
type LLDPNeighborStatus = typed.Resource[LLDPNeighborStatusSpec, LLDPNeighborStatusExtension]

// LLDPNeighborStatusSpec contains LLDP neighbors observed on a link.
//
//gotagsrewrite:gen
type LLDPNeighborStatusSpec struct {
	Neighbors []LLDPNeighborSpec `yaml:"neighbors,omitempty" protobuf:"1"`
}

// LLDPNeighborSpec describes the advertised identity of one neighbor.
//
//gotagsrewrite:gen
type LLDPNeighborSpec struct {
	ChassisID           string         `yaml:"chassisID" protobuf:"1"`
	SystemName          string         `yaml:"systemName,omitempty" protobuf:"2"`
	SystemDescription   string         `yaml:"systemDescription,omitempty" protobuf:"3"`
	PortID              string         `yaml:"portID" protobuf:"4"`
	PortDescription     string         `yaml:"portDescription,omitempty" protobuf:"5"`
	ManagementAddresses []string       `yaml:"managementAddresses,omitempty" protobuf:"6"`
	VLANs               []LLDPVLANSpec `yaml:"vlans,omitempty" protobuf:"7"`
}

// LLDPVLANSpec describes an advertised VLAN.
//
//gotagsrewrite:gen
type LLDPVLANSpec struct {
	ID   uint16 `yaml:"id" protobuf:"1"`
	Name string `yaml:"name,omitempty" protobuf:"2"`
}

// NewLLDPNeighborStatus initializes the LLDP status for a link.
func NewLLDPNeighborStatus(namespace resource.Namespace, id resource.ID) *LLDPNeighborStatus {
	return typed.NewResource[LLDPNeighborStatusSpec, LLDPNeighborStatusExtension](
		resource.NewMetadata(namespace, LLDPNeighborStatusType, id, resource.VersionUndefined),
		LLDPNeighborStatusSpec{},
	)
}

// LLDPNeighborStatusExtension provides auxiliary methods for LLDPNeighborStatus.
type LLDPNeighborStatusExtension struct{}

// ResourceDefinition implements [typed.Extension].
func (LLDPNeighborStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LLDPNeighborStatusType,
		Aliases:          []resource.Type{"lldp"},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[LLDPNeighborStatusSpec](LLDPNeighborStatusType, &LLDPNeighborStatus{}); err != nil {
		panic(err)
	}
}
