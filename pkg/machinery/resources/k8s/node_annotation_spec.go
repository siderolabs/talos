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

// NodeAnnotationSpecType is the type.
const NodeAnnotationSpecType = resource.Type("NodeAnnotationSpecs.k8s.talos.dev")

// NodeAnnotationSpecSpec represents an annoation that's attached to a Talos node.
//
//gotagsrewrite:gen
type NodeAnnotationSpecSpec struct {
	Key   string `yaml:"key" protobuf:"1"`
	Value string `yaml:"value" protobuf:"2"`
}

// NodeAnnotationSpec ...
type NodeAnnotationSpec = typed.Resource[NodeAnnotationSpecSpec, NodeAnnotationSpecExtension]

// NewNodeAnnotationSpec initializes a NodeAnnotation resource.
func NewNodeAnnotationSpec(id resource.ID) *NodeAnnotationSpec {
	return typed.NewResource[NodeAnnotationSpecSpec, NodeAnnotationSpecExtension](
		resource.NewMetadata(NamespaceName, NodeAnnotationSpecType, id, resource.VersionUndefined),
		NodeAnnotationSpecSpec{},
	)
}

// NodeAnnotationSpecExtension provides auxiliary methods for NodeAnnotation.
type NodeAnnotationSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (NodeAnnotationSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeAnnotationSpecType,
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

	err := protobuf.RegisterDynamic[NodeAnnotationSpecSpec](NodeAnnotationSpecType, &NodeAnnotationSpec{})
	if err != nil {
		panic(err)
	}
}
