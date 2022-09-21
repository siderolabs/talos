// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/gen/slices"

	"github.com/talos-systems/talos/pkg/machinery/proto"
)

// NodeAddressType is type of NodeAddress resource.
const NodeAddressType = resource.Type("NodeAddresses.net.talos.dev")

// NodeAddress resource holds physical network link status.
type NodeAddress = typed.Resource[NodeAddressSpec, NodeAddressRD]

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
	// Routed current node addresses (as seen at the moment).
	//
	// This is current addresses minus 'external' IPs, and SideroLink IPs.
	//
	// This list is used to pick advertised/listen addresses for different services.
	NodeAddressRoutedID = "routed"
)

// NodeAddressSpec describes a set of node addresses.
//
//gotagsrewrite:gen
type NodeAddressSpec struct {
	Addresses []netip.Prefix `yaml:"addresses" protobuf:"1"`
}

// NewNodeAddress initializes a NodeAddress resource.
func NewNodeAddress(namespace resource.Namespace, id resource.ID) *NodeAddress {
	return typed.NewResource[NodeAddressSpec, NodeAddressRD](
		resource.NewMetadata(namespace, NodeAddressType, id, resource.VersionUndefined),
		NodeAddressSpec{},
	)
}

// NodeAddressRD provides auxiliary methods for NodeAddress.
type NodeAddressRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (NodeAddressRD) ResourceDefinition(resource.Metadata, NodeAddressSpec) meta.ResourceDefinitionSpec {
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

// IPs returns IP without prefix.
func (spec *NodeAddressSpec) IPs() []netip.Addr {
	return slices.Map(spec.Addresses, netip.Prefix.Addr)
}

// FilteredNodeAddressID returns resource ID for node addresses with filter applied.
func FilteredNodeAddressID(kind resource.ID, filterID string) resource.ID {
	return fmt.Sprintf("%s-%s", kind, filterID)
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[NodeAddressSpec](NodeAddressType, &NodeAddress{})
	if err != nil {
		panic(err)
	}
}
