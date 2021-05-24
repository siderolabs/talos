// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
)

// AddressStatusType is type of AddressStatus resource.
const AddressStatusType = resource.Type("AddressStatuses.net.talos.dev")

// AddressStatus resource holds physical network link status.
type AddressStatus struct {
	md   resource.Metadata
	spec AddressStatusSpec
}

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

// NewAddressStatus initializes a SecretsStatus resource.
func NewAddressStatus(namespace resource.Namespace, id resource.ID) *AddressStatus {
	r := &AddressStatus{
		md:   resource.NewMetadata(namespace, AddressStatusType, id, resource.VersionUndefined),
		spec: AddressStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *AddressStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *AddressStatus) Spec() interface{} {
	return r.spec
}

func (r *AddressStatus) String() string {
	return fmt.Sprintf("network.AddressStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *AddressStatus) DeepCopy() resource.Resource {
	return &AddressStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *AddressStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AddressStatusType,
		Aliases:          []resource.Type{"address", "addresses"},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// Status sets pod status.
func (r *AddressStatus) Status() *AddressStatusSpec {
	return &r.spec
}
