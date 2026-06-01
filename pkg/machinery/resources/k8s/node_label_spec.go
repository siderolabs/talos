// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s //nolint:dupl

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// NodeLabelSpecType is the type.
const NodeLabelSpecType = resource.Type("NodeLabelSpecs.k8s.talos.dev")

// NodeLabelSpecSpec represents a label that's attached to a Talos node.
//
//gotagsrewrite:gen
type NodeLabelSpecSpec struct {
	Key   string `yaml:"key" protobuf:"1"`
	Value string `yaml:"value" protobuf:"2"`
}

// NodeLabelSpec ...
type NodeLabelSpec = typed.Resource[NodeLabelSpecSpec, NodeLabelSpecExtension]

// NewNodeLabelSpec initializes a NodeLabel resource.
func NewNodeLabelSpec(id resource.ID) *NodeLabelSpec {
	return typed.NewResource[NodeLabelSpecSpec, NodeLabelSpecExtension](
		resource.NewMetadata(NamespaceName, NodeLabelSpecType, id, resource.VersionUndefined),
		NodeLabelSpecSpec{},
	)
}

// NodeLabelSpecExtension provides auxiliary methods for NodeLabel.
type NodeLabelSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (NodeLabelSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeLabelSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Value",
				JSONPath: "{.value}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[NodeLabelSpecSpec](NodeLabelSpecType, &NodeLabelSpec{})
	if err != nil {
		panic(err)
	}
}
