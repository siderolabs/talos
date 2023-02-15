// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// NodeIPConfigType is type of NodeIPConfig resource.
const NodeIPConfigType = resource.Type("NodeIPConfigs.kubernetes.talos.dev")

// NodeIPConfig resource holds definition of Node IP specification.
type NodeIPConfig = typed.Resource[NodeIPConfigSpec, NodeIPConfigExtension]

// NodeIPConfigSpec holds the Node IP specification.
//
//gotagsrewrite:gen
type NodeIPConfigSpec struct {
	ValidSubnets   []string `yaml:"validSubnets,omitempty" protobuf:"1"`
	ExcludeSubnets []string `yaml:"excludeSubnets" protobuf:"2"`
}

// NewNodeIPConfig initializes an empty NodeIPConfig resource.
func NewNodeIPConfig(namespace resource.Namespace, id resource.ID) *NodeIPConfig {
	return typed.NewResource[NodeIPConfigSpec, NodeIPConfigExtension](
		resource.NewMetadata(namespace, NodeIPConfigType, id, resource.VersionUndefined),
		NodeIPConfigSpec{},
	)
}

// NodeIPConfigExtension provides auxiliary methods for NodeIPConfig.
type NodeIPConfigExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (NodeIPConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeIPConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[NodeIPConfigSpec](NodeIPConfigType, &NodeIPConfig{})
	if err != nil {
		panic(err)
	}
}
