// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// LinkSpecType is type of LinkSpec resource.
const LinkSpecType = resource.Type("LinkSpecs.net.talos.dev")

// LinkSpec resource describes desired state of the link (network interface).
type LinkSpec struct {
	md   resource.Metadata
	spec LinkSpecSpec
}

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
	r := &LinkSpec{
		md:   resource.NewMetadata(namespace, LinkSpecType, id, resource.VersionUndefined),
		spec: LinkSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *LinkSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *LinkSpec) Spec() interface{} {
	return r.spec
}

func (r *LinkSpec) String() string {
	return fmt.Sprintf("network.LinkSpec(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *LinkSpec) DeepCopy() resource.Resource {
	return &LinkSpec{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *LinkSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             LinkSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
		Sensitivity:      meta.Sensitive,
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *LinkSpec) TypedSpec() *LinkSpecSpec {
	return &r.spec
}
