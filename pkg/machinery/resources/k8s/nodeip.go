// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"
)

// NodeIPType is type of NodeIP resource.
const NodeIPType = resource.Type("NodeIPs.kubernetes.talos.dev")

// NodeIP resource holds definition of Node IP specification.
type NodeIP struct {
	md   resource.Metadata
	spec *NodeIPSpec
}

// NodeIPSpec holds the Node IP specification.
type NodeIPSpec struct {
	Addresses []netaddr.IP `yaml:"addresses"`
}

// NewNodeIP initializes an empty NodeIP resource.
func NewNodeIP(namespace resource.Namespace, id resource.ID) *NodeIP {
	r := &NodeIP{
		md:   resource.NewMetadata(namespace, NodeIPType, id, resource.VersionUndefined),
		spec: &NodeIPSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *NodeIP) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *NodeIP) Spec() interface{} {
	return r.spec
}

func (r *NodeIP) String() string {
	return fmt.Sprintf("k8s.NodeIP(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *NodeIP) DeepCopy() resource.Resource {
	return &NodeIP{
		md: r.md,
		spec: &NodeIPSpec{
			Addresses: append([]netaddr.IP(nil), r.spec.Addresses...),
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *NodeIP) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeIPType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

// TypedSpec returns .spec.
func (r *NodeIP) TypedSpec() *NodeIPSpec {
	return r.spec
}
