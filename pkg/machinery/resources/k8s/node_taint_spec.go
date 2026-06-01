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

// NodeTaintSpecType is the type.
const NodeTaintSpecType = resource.Type("NodeTaintSpecs.k8s.talos.dev")

// NodeTaintSpecSpec represents a label that's attached to a Talos node.
//
//gotagsrewrite:gen
type NodeTaintSpecSpec struct {
	Key    string `yaml:"key" protobuf:"1"`
	Effect string `yaml:"effect" protobuf:"2"`
	Value  string `yaml:"value" protobuf:"3"`
}

// NodeTaintSpec ...
type NodeTaintSpec = typed.Resource[NodeTaintSpecSpec, NodeTaintSpecExtension]

// NewNodeTaintSpec initializes a NodeLabel resource.
func NewNodeTaintSpec(id resource.ID) *NodeTaintSpec {
	return typed.NewResource[NodeTaintSpecSpec, NodeTaintSpecExtension](
		resource.NewMetadata(NamespaceName, NodeTaintSpecType, id, resource.VersionUndefined),
		NodeTaintSpecSpec{},
	)
}

// NodeTaintSpecExtension provides auxiliary methods for NodeLabel.
type NodeTaintSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (NodeTaintSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeTaintSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Effect",
				JSONPath: "{.effect}",
			},
			{
				Name:     "Value",
				JSONPath: "{.value}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[NodeTaintSpecSpec](NodeTaintSpecType, &NodeTaintSpec{})
	if err != nil {
		panic(err)
	}
}
