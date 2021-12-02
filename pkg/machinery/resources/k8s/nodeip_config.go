// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// NodeIPConfigType is type of NodeIPConfig resource.
const NodeIPConfigType = resource.Type("NodeIPConfigs.kubernetes.talos.dev")

// NodeIPConfig resource holds definition of Node IP specification.
type NodeIPConfig struct {
	md   resource.Metadata
	spec *NodeIPConfigSpec
}

// NodeIPConfigSpec holds the Node IP specification.
type NodeIPConfigSpec struct {
	ValidSubnets   []string `yaml:"validSubnets,omitempty"`
	ExcludeSubnets []string `yaml:"excludeSubnets"`
}

// NewNodeIPConfig initializes an empty NodeIPConfig resource.
func NewNodeIPConfig(namespace resource.Namespace, id resource.ID) *NodeIPConfig {
	r := &NodeIPConfig{
		md:   resource.NewMetadata(namespace, NodeIPConfigType, id, resource.VersionUndefined),
		spec: &NodeIPConfigSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *NodeIPConfig) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *NodeIPConfig) Spec() interface{} {
	return r.spec
}

func (r *NodeIPConfig) String() string {
	return fmt.Sprintf("k8s.NodeIPConfig(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *NodeIPConfig) DeepCopy() resource.Resource {
	return &NodeIPConfig{
		md: r.md,
		spec: &NodeIPConfigSpec{
			ValidSubnets:   append([]string(nil), r.spec.ValidSubnets...),
			ExcludeSubnets: append([]string(nil), r.spec.ExcludeSubnets...),
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *NodeIPConfig) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeIPConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

// TypedSpec returns .spec.
func (r *NodeIPConfig) TypedSpec() *NodeIPConfigSpec {
	return r.spec
}
