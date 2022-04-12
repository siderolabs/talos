// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// AddressSpecType is type of AddressSpec resource.
const AddressSpecType = resource.Type("AddressSpecs.net.talos.dev")

// AddressSpec resource holds physical network link status.
type AddressSpec struct {
	md   resource.Metadata
	spec AddressSpecSpec
}

// AddressSpecSpec describes status of rendered secrets.
type AddressSpecSpec struct {
	Address         netaddr.IPPrefix        `yaml:"address"`
	LinkName        string                  `yaml:"linkName"`
	Family          nethelpers.Family       `yaml:"family"`
	Scope           nethelpers.Scope        `yaml:"scope"`
	Flags           nethelpers.AddressFlags `yaml:"flags"`
	AnnounceWithARP bool                    `yaml:"announceWithARP,omitempty"`
	ConfigLayer     ConfigLayer             `yaml:"layer"`
}

// NewAddressSpec initializes a AddressSpec resource.
func NewAddressSpec(namespace resource.Namespace, id resource.ID) *AddressSpec {
	r := &AddressSpec{
		md:   resource.NewMetadata(namespace, AddressSpecType, id, resource.VersionUndefined),
		spec: AddressSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *AddressSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *AddressSpec) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *AddressSpec) DeepCopy() resource.Resource {
	return &AddressSpec{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *AddressSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AddressSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *AddressSpec) TypedSpec() *AddressSpecSpec {
	return &r.spec
}
