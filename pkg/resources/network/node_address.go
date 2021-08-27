// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"
)

// NodeAddressType is type of NodeAddress resource.
const NodeAddressType = resource.Type("NodeAddresses.net.talos.dev")

// NodeAddress resource holds physical network link status.
type NodeAddress struct {
	md   resource.Metadata
	spec NodeAddressSpec
}

// NodeAddress well-known IDs.
const (
	// Default node address (should be a single address in the spec).
	//
	// Used to set for example published etcd peer address.
	NodeAddressDefaultID = "default"
	// Current node addresses (as seen at the moment).
	//
	// Shows a list of addresses for the node for the UP interfaces.
	NodeAddressCurrentID = "current"
	// Accumulative list of the addresses node had over time.
	//
	// If some address is no longer present, it will be still kept in the accumulative list.
	NodeAddressAccumulativeID = "accumulative"
)

// NodeAddressSpec describes a set of node addresses.
type NodeAddressSpec struct {
	Addresses []netaddr.IP `yaml:"addresses"`
}

// NewNodeAddress initializes a NodeAddress resource.
func NewNodeAddress(namespace resource.Namespace, id resource.ID) *NodeAddress {
	r := &NodeAddress{
		md:   resource.NewMetadata(namespace, NodeAddressType, id, resource.VersionUndefined),
		spec: NodeAddressSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *NodeAddress) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *NodeAddress) Spec() interface{} {
	return r.spec
}

func (r *NodeAddress) String() string {
	return fmt.Sprintf("network.NodeAddress(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *NodeAddress) DeepCopy() resource.Resource {
	return &NodeAddress{
		md: r.md,
		spec: NodeAddressSpec{
			Addresses: append([]netaddr.IP(nil), r.spec.Addresses...),
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *NodeAddress) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeAddressType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Addresses",
				JSONPath: `{.addresses}`,
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *NodeAddress) TypedSpec() *NodeAddressSpec {
	return &r.spec
}

// FilteredNodeAddressID returns resource ID for node addresses with filter applied.
func FilteredNodeAddressID(kind resource.ID, filterID string) resource.ID {
	return fmt.Sprintf("%s-%s", kind, filterID)
}
