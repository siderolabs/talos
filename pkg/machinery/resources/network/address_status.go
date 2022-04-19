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

// AddressStatusType is type of AddressStatus resource.
const AddressStatusType = resource.Type("AddressStatuses.net.talos.dev")

// AddressStatus resource holds physical network link status.
type AddressStatus = typed.Resource[AddressStatusSpec, AddressStatusRD]

// AddressStatusSpec describes status of rendered secrets.
type AddressStatusSpec struct {
	Address   netaddr.IPPrefix        `yaml:"address"`
	Local     netaddr.IP              `yaml:"local,omitempty"`
	Broadcast netaddr.IP              `yaml:"broadcast,omitempty"`
	Anycast   netaddr.IP              `yaml:"anycast,omitempty"`
	Multicast netaddr.IP              `yaml:"multicast,omitempty"`
	LinkIndex uint32                  `yaml:"linkIndex"`
	LinkName  string                  `yaml:"linkName"`
	Family    nethelpers.Family       `yaml:"family"`
	Scope     nethelpers.Scope        `yaml:"scope"`
	Flags     nethelpers.AddressFlags `yaml:"flags"`
}

// DeepCopy generates a deep copy of AddressStatusSpec.
func (spec AddressStatusSpec) DeepCopy() AddressStatusSpec {
	return spec
}

// NewAddressStatus initializes a AddressStatus resource.
func NewAddressStatus(namespace resource.Namespace, id resource.ID) *AddressStatus {
	return typed.NewResource[AddressStatusSpec, AddressStatusRD](
		resource.NewMetadata(namespace, AddressStatusType, id, resource.VersionUndefined),
		AddressStatusSpec{},
	)
}

// AddressStatusRD provides auxiliary methods for AddressStatus.
type AddressStatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (AddressStatusRD) ResourceDefinition(resource.Metadata, AddressStatusSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AddressStatusType,
		Aliases:          []resource.Type{"address", "addresses"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Address",
				JSONPath: `{.address}`,
			},
			{
				Name:     "Link",
				JSONPath: `{.linkName}`,
			},
		},
	}
}
