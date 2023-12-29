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

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

//go:generate deep-copy -type AddressSpecSpec -type AddressStatusSpec -type DNSResolveCacheSpec -type HardwareAddrSpec -type HostnameSpecSpec -type HostnameStatusSpec -type LinkRefreshSpec -type LinkSpecSpec -type LinkStatusSpec -type NfTablesChainSpec -type NodeAddressSpec -type NodeAddressFilterSpec -type OperatorSpecSpec -type ProbeSpecSpec -type ProbeStatusSpec -type ResolverSpecSpec -type ResolverStatusSpec -type RouteSpecSpec -type RouteStatusSpec -type StatusSpec -type TimeServerSpecSpec -type TimeServerStatusSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// AddressSpecType is type of AddressSpec resource.
const AddressSpecType = resource.Type("AddressSpecs.net.talos.dev")

// AddressSpec resource holds physical network link status.
type AddressSpec = typed.Resource[AddressSpecSpec, AddressSpecExtension]

// AddressSpecSpec describes status of rendered secrets.
//
//gotagsrewrite:gen
type AddressSpecSpec struct {
	Address         netip.Prefix            `yaml:"address" protobuf:"1"`
	LinkName        string                  `yaml:"linkName" protobuf:"2"`
	Family          nethelpers.Family       `yaml:"family" protobuf:"3"`
	Scope           nethelpers.Scope        `yaml:"scope" protobuf:"4"`
	Flags           nethelpers.AddressFlags `yaml:"flags" protobuf:"5"`
	AnnounceWithARP bool                    `yaml:"announceWithARP,omitempty" protobuf:"6"`
	ConfigLayer     ConfigLayer             `yaml:"layer" protobuf:"7"`
}

// NewAddressSpec initializes a AddressSpec resource.
func NewAddressSpec(namespace resource.Namespace, id resource.ID) *AddressSpec {
	return typed.NewResource[AddressSpecSpec, AddressSpecExtension](
		resource.NewMetadata(namespace, AddressSpecType, id, resource.VersionUndefined),
		AddressSpecSpec{},
	)
}

// AddressSpecExtension provides auxiliary methods for AddressSpec.
type AddressSpecExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (AddressSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AddressSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[AddressSpecSpec](AddressSpecType, &AddressSpec{})
	if err != nil {
		panic(err)
	}
}
