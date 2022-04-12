// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"
)

// NodeAddressFilterType is type of NodeAddressFilter resource.
const NodeAddressFilterType = resource.Type("NodeAddressFilters.net.talos.dev")

// NodeAddressFilter resource holds filter for NodeAddress resources.
type NodeAddressFilter struct {
	md   resource.Metadata
	spec NodeAddressFilterSpec
}

// NodeAddressFilterSpec describes a filter for NodeAddresses.
type NodeAddressFilterSpec struct {
	// Address is skipped if it doesn't match any of the includeSubnets (if includeSubnets is not empty).
	IncludeSubnets []netaddr.IPPrefix `yaml:"includeSubnets"`
	// Address is skipped if it matches any of the includeSubnets.
	ExcludeSubnets []netaddr.IPPrefix `yaml:"excludeSubnets"`
}

// NewNodeAddressFilter initializes a NodeAddressFilter resource.
func NewNodeAddressFilter(namespace resource.Namespace, id resource.ID) *NodeAddressFilter {
	r := &NodeAddressFilter{
		md:   resource.NewMetadata(namespace, NodeAddressFilterType, id, resource.VersionUndefined),
		spec: NodeAddressFilterSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *NodeAddressFilter) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *NodeAddressFilter) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *NodeAddressFilter) DeepCopy() resource.Resource {
	return &NodeAddressFilter{
		md: r.md,
		spec: NodeAddressFilterSpec{
			IncludeSubnets: append([]netaddr.IPPrefix(nil), r.spec.IncludeSubnets...),
			ExcludeSubnets: append([]netaddr.IPPrefix(nil), r.spec.ExcludeSubnets...),
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *NodeAddressFilter) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeAddressFilterType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Include Subnets",
				JSONPath: `{.includeSubnets}`,
			},
			{
				Name:     "Exclude Subnets",
				JSONPath: `{.excludeSubnets}`,
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *NodeAddressFilter) TypedSpec() *NodeAddressFilterSpec {
	return &r.spec
}
