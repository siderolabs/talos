// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// LinkSpecType is type of LinkSpec resource.
const LinkSpecType = resource.Type("LinkSpecs.net.talos.dev")

// LinkSpec resource describes desired state of the link (network interface).
type LinkSpec = typed.Resource[LinkSpecSpec, LinkSpecRD]

// LinkSpecSpec describes spec for the link.
type LinkSpecSpec struct {
	// Name defines link name
	Name string `yaml:"name"`

	// Logical describes if the interface should be created on the fly if it doesn't exist.
	Logical bool `yaml:"logical"`

	// If Up is true, bring interface up, otherwise bring interface down.
	//
	// TODO: make *bool ?
	Up bool `yaml:"up"`

	// Interface MTU (always applies).
	MTU uint32 `yaml:"mtu"`

	// Kind and Type are only required for Logical interfaces.
	Kind string              `yaml:"kind"`
	Type nethelpers.LinkType `yaml:"type"`

	// ParentName indicates link parent for VLAN interfaces.
	ParentName string `yaml:"parentName,omitempty"`

	// MasterName indicates master link for enslaved bonded interfaces.
	BondSlave BondSlave `yaml:",omitempty,inline"`

	// These structures are present depending on "Kind" for Logical intefaces.
	VLAN       VLANSpec       `yaml:"vlan,omitempty"`
	BondMaster BondMasterSpec `yaml:"bondMaster,omitempty"`
	Wireguard  WireguardSpec  `yaml:"wireguard,omitempty"`

	// Configuration layer.
	ConfigLayer ConfigLayer `yaml:"layer"`
}

// BondSlave contains a bond's master name and slave index.
type BondSlave struct {
	// MasterName indicates master link for enslaved bonded interfaces.
	MasterName string `yaml:"masterName,omitempty"`

	// SlaveIndex indicates a slave's position in bond.
	SlaveIndex int `yaml:"slaveIndex,omitempty"`
}

// Merge with other, overwriting fields from other if set.
//
//nolint:gocyclo
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
	updateIfNotZero(&spec.BondMaster, other.BondMaster)

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
	return typed.NewResource[LinkSpecSpec, LinkSpecRD](
		resource.NewMetadata(namespace, LinkSpecType, id, resource.VersionUndefined),
		LinkSpecSpec{},
	)
}

// LinkSpecRD provides auxiliary methods for LinkSpec.
type LinkSpecRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (LinkSpecRD) ResourceDefinition(resource.Metadata, LinkSpecSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LinkSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}
