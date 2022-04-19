// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"inet.af/netaddr"

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
	MasterName string `yaml:"masterName,omitempty"`

	// These structures are present depending on "Kind" for Logical intefaces.
	VLAN       VLANSpec       `yaml:"vlan,omitempty"`
	BondMaster BondMasterSpec `yaml:"bondMaster,omitempty"`
	Wireguard  WireguardSpec  `yaml:"wireguard,omitempty"`

	// Configuration layer.
	ConfigLayer ConfigLayer `yaml:"layer"`
}

// DeepCopy generates a deep copy of LinkSpecSpec.
func (spec LinkSpecSpec) DeepCopy() LinkSpecSpec {
	cp := spec
	if spec.Wireguard.Peers != nil {
		cp.Wireguard.Peers = make([]WireguardPeer, len(spec.Wireguard.Peers))
		copy(cp.Wireguard.Peers, spec.Wireguard.Peers)

		for i3 := range spec.Wireguard.Peers {
			if spec.Wireguard.Peers[i3].AllowedIPs != nil {
				cp.Wireguard.Peers[i3].AllowedIPs = make([]netaddr.IPPrefix, len(spec.Wireguard.Peers[i3].AllowedIPs))
				copy(cp.Wireguard.Peers[i3].AllowedIPs, spec.Wireguard.Peers[i3].AllowedIPs)
			}
		}
	}

	return cp
}

var (
	zeroVLAN       VLANSpec
	zeroBondMaster BondMasterSpec
)

// Merge with other, overwriting fields from other if set.
//
//nolint:gocyclo
func (spec *LinkSpecSpec) Merge(other *LinkSpecSpec) error {
	// prefer Logical, as it is defined for bonds/vlans, etc.
	if other.Logical {
		spec.Logical = other.Logical
	}

	if other.Up {
		spec.Up = other.Up
	}

	if other.MTU != 0 {
		spec.MTU = other.MTU
	}

	if other.Kind != "" {
		spec.Kind = other.Kind
	}

	if other.Type != 0 {
		spec.Type = other.Type
	}

	if other.ParentName != "" {
		spec.ParentName = other.ParentName
	}

	if other.MasterName != "" {
		spec.MasterName = other.MasterName
	}

	if other.VLAN != zeroVLAN {
		spec.VLAN = other.VLAN
	}

	if other.BondMaster != zeroBondMaster {
		spec.BondMaster = other.BondMaster
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
