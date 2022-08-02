// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

// NodeIPConfigType is type of NodeIPConfig resource.
const NodeIPConfigType = resource.Type("NodeIPConfigs.kubernetes.talos.dev")

// NodeIPConfig resource holds definition of Node IP specification.
type NodeIPConfig = typed.Resource[NodeIPConfigSpec, NodeIPConfigRD]

// NodeIPConfigSpec holds the Node IP specification.
//
//gotagsrewrite:gen
type NodeIPConfigSpec struct {
	ValidSubnets   []string `yaml:"validSubnets,omitempty" protobuf:"1"`
	ExcludeSubnets []string `yaml:"excludeSubnets" protobuf:"2"`
}

// NewNodeIPConfig initializes an empty NodeIPConfig resource.
func NewNodeIPConfig(namespace resource.Namespace, id resource.ID) *NodeIPConfig {
	return typed.NewResource[NodeIPConfigSpec, NodeIPConfigRD](
		resource.NewMetadata(namespace, NodeIPConfigType, id, resource.VersionUndefined),
		NodeIPConfigSpec{},
	)
}

// NodeIPConfigRD provides auxiliary methods for NodeIPConfig.
type NodeIPConfigRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (NodeIPConfigRD) ResourceDefinition(resource.Metadata, NodeIPConfigSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeIPConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}
