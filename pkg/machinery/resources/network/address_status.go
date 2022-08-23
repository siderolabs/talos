// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/proto"
)

// AddressStatusType is type of AddressStatus resource.
const AddressStatusType = resource.Type("AddressStatuses.net.talos.dev")

// AddressStatus resource holds physical network link status.
type AddressStatus = typed.Resource[AddressStatusSpec, AddressStatusRD]

// AddressStatusSpec describes status of rendered secrets.
//
//gotagsrewrite:gen
type AddressStatusSpec struct {
	Address   netip.Prefix            `yaml:"address" protobuf:"1"`
	Local     netip.Addr              `yaml:"local,omitempty" protobuf:"2"`
	Broadcast netip.Addr              `yaml:"broadcast,omitempty" protobuf:"3"`
	Anycast   netip.Addr              `yaml:"anycast,omitempty" protobuf:"4"`
	Multicast netip.Addr              `yaml:"multicast,omitempty" protobuf:"5"`
	LinkIndex uint32                  `yaml:"linkIndex" protobuf:"6"`
	LinkName  string                  `yaml:"linkName" protobuf:"7"`
	Family    nethelpers.Family       `yaml:"family" protobuf:"8"`
	Scope     nethelpers.Scope        `yaml:"scope" protobuf:"9"`
	Flags     nethelpers.AddressFlags `yaml:"flags" protobuf:"10"`
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

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[AddressStatusSpec](AddressStatusType, &AddressStatus{})
	if err != nil {
		panic(err)
	}
}
