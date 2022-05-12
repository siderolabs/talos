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

//nolint:lll
//go:generate deep-copy -type AddressSpecSpec -type AddressStatusSpec -type HardwareAddrSpec -type HostnameSpecSpec -type HostnameStatusSpec -type LinkRefreshSpec -type LinkSpecSpec -type LinkStatusSpec -type NodeAddressSpec -type NodeAddressFilterSpec -type OperatorSpecSpec -type ResolverSpecSpec -type ResolverStatusSpec -type RouteSpecSpec -type RouteStatusSpec -type StatusSpec -type TimeServerSpecSpec -type TimeServerStatusSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// AddressSpecType is type of AddressSpec resource.
const AddressSpecType = resource.Type("AddressSpecs.net.talos.dev")

// AddressSpec resource holds physical network link status.
type AddressSpec = typed.Resource[AddressSpecSpec, AddressSpecRD]

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
	return typed.NewResource[AddressSpecSpec, AddressSpecRD](
		resource.NewMetadata(namespace, AddressSpecType, id, resource.VersionUndefined),
		AddressSpecSpec{},
	)
}

// AddressSpecRD provides auxiliary methods for AddressSpec.
type AddressSpecRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (AddressSpecRD) ResourceDefinition(resource.Metadata, AddressSpecSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AddressSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}
