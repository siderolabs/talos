// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// LinkSpecType is type of LinkSpec resource.
const LinkSpecType = resource.Type("LinkSpecs.net.talos.dev")

// LinkSpec resource describes desired state of the link (network interface).
type LinkSpec = typed.Resource[LinkSpecSpec, LinkSpecExtension]

// LinkSpecSpec describes spec for the link.
//
//gotagsrewrite:gen
type LinkSpecSpec struct {
	// Name defines link name
	Name string `yaml:"name" protobuf:"1"`

	// Logical describes if the interface should be created on the fly if it doesn't exist.
	Logical bool `yaml:"logical" protobuf:"2"`

	// If Up is true, bring interface up, otherwise bring interface down.
	//
	// TODO: make *bool ?
	Up bool `yaml:"up" protobuf:"3"`

	// Interface MTU (always applies).
	MTU uint32 `yaml:"mtu" protobuf:"4"`

	// Kind and Type are only required for Logical interfaces.
	Kind string              `yaml:"kind" protobuf:"5"`
	Type nethelpers.LinkType `yaml:"type" protobuf:"6"`

	// Override hardware (MAC) address (if supported).
	HardwareAddress nethelpers.HardwareAddr `yaml:"hardwareAddr,omitempty" protobuf:"15"`

	// ParentName indicates link parent for VLAN interfaces.
	ParentName string `yaml:"parentName,omitempty" protobuf:"7"`

	// MasterName indicates master link for enslaved bonded interfaces.
	BondSlave BondSlave `yaml:",omitempty,inline" protobuf:"8"`

	// BridgeSlave indicates master link for bridged interfaces.
	BridgeSlave BridgeSlave `yaml:"bridgeSlave,omitempty" protobuf:"9"`

	// VRFSlave indicates master link for interfaces in a vrf
	VRFSlave VRFSlave `yaml:"vrfSlave,omitempty" protobuf:"18"`

	// These structures are present depending on "Kind" for Logical interfaces.
	VLAN         VLANSpec         `yaml:"vlan,omitempty" protobuf:"10"`
	BondMaster   BondMasterSpec   `yaml:"bondMaster,omitempty" protobuf:"11"`
	BridgeMaster BridgeMasterSpec `yaml:"bridgeMaster,omitempty" protobuf:"12"`
	VRFMaster    VRFMasterSpec    `yaml:"vrfMaster,omitempty" protobuf:"17"`
	Wireguard    WireguardSpec    `yaml:"wireguard,omitempty" protobuf:"13"`

	// Configuration layer.
	ConfigLayer ConfigLayer `yaml:"layer" protobuf:"14"`

	// Multicast indicates whether the multicast flag should be set on the interface to the value.
	Multicast *bool `yaml:"multicast,omitempty" protobuf:"16"`
}

// BondSlave contains a bond's master name and slave index.
//
//gotagsrewrite:gen
type BondSlave struct {
	// MasterName indicates master link for enslaved bonded interfaces.
	MasterName string `yaml:"masterName,omitempty" protobuf:"1"`

	// SlaveIndex indicates a slave's position in bond.
	SlaveIndex int `yaml:"slaveIndex,omitempty" protobuf:"2"`
}

// BridgeSlave contains the name of the master bridge of a bridged interface
//
//gotagsrewrite:gen
type BridgeSlave struct {
	// MasterName indicates master link for enslaved bridged interfaces.
	MasterName string `yaml:"masterName,omitempty" protobuf:"1"`
}

// VRFSlave contains the name of the master vrf for an interface
//
//gotagsrewrite:gen
type VRFSlave struct {
	MasterName string `yaml:"masterName,omitempty" protobuf:"1"`
}

// Merge with other, overwriting fields from other if set.
func (spec *LinkSpecSpec) Merge(other *LinkSpecSpec) error {
	// prefer Logical, as it is defined for bonds/vlans, etc.
	updateIfNotZero(&spec.Logical, other.Logical)
	updateIfNotZero(&spec.Up, other.Up)
	updateIfNotZero(&spec.MTU, other.MTU)
	updateIfNotZero(&spec.Kind, other.Kind)
	updateIfNotZero(&spec.Type, other.Type)
	updateIfNotZero(&spec.ParentName, other.ParentName)
	updateIfNotZero(&spec.BondSlave, other.BondSlave)
	updateIfNotZero(&spec.VLAN, other.VLAN)
	updateIfNotZero(&spec.BridgeMaster, other.BridgeMaster)
	updateIfNotZero(&spec.BridgeSlave, other.BridgeSlave)
	updateIfNotZero(&spec.VRFMaster, other.VRFMaster)
	updateIfNotZero(&spec.VRFSlave, other.VRFSlave)

	if !other.BondMaster.IsZero() {
		spec.BondMaster = other.BondMaster.DeepCopy()
	}

	if other.HardwareAddress != nil {
		spec.HardwareAddress = slices.Clone(other.HardwareAddress)
	}

	// Wireguard config should be able to apply non-zero values in earlier config layers which may be zero values in later layers.
	// Thus, we handle each Wireguard configuration value discretely.
	if !other.Wireguard.IsZero() {
		if spec.Wireguard.IsZero() {
			spec.Wireguard = other.Wireguard
		} else {
			spec.Wireguard.Merge(other.Wireguard)
		}
	}

	spec.ConfigLayer = other.ConfigLayer

	return nil
}

func updateIfNotZero[T comparable](target *T, source T) {
	var zero T
	if source != zero {
		*target = source
	}
}

// NewLinkSpec initializes a LinkSpec resource.
func NewLinkSpec(namespace resource.Namespace, id resource.ID) *LinkSpec {
	return typed.NewResource[LinkSpecSpec, LinkSpecExtension](
		resource.NewMetadata(namespace, LinkSpecType, id, resource.VersionUndefined),
		LinkSpecSpec{},
	)
}

// LinkSpecExtension provides auxiliary methods for LinkSpec.
type LinkSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (LinkSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LinkSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[LinkSpecSpec](LinkSpecType, &LinkSpec{})
	if err != nil {
		panic(err)
	}
}
