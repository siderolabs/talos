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

// NodeCordonedSpecType is the type.
const NodeCordonedSpecType = resource.Type("NodeCordonedSpecs.k8s.talos.dev")

// NodeCordonedSpecSpec represents an intention to make a node cordoned (unschedulable).
//
//gotagsrewrite:gen
type NodeCordonedSpecSpec struct{}

// NodeCordonedID is the ID of the NodeCordonedSpec resource.
const NodeCordonedID = resource.ID("cordoned")

// NodeCordonedSpec ...
type NodeCordonedSpec = typed.Resource[NodeCordonedSpecSpec, NodeCordonedSpecExtension]

// NewNodeCordonedSpec initializes a NodeLabel resource.
func NewNodeCordonedSpec(id resource.ID) *NodeCordonedSpec {
	return typed.NewResource[NodeCordonedSpecSpec, NodeCordonedSpecExtension](
		resource.NewMetadata(NamespaceName, NodeCordonedSpecType, id, resource.VersionUndefined),
		NodeCordonedSpecSpec{},
	)
}

// NodeCordonedSpecExtension provides auxiliary methods for NodeLabel.
type NodeCordonedSpecExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (NodeCordonedSpecExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeCordonedSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[NodeCordonedSpecSpec](NodeCordonedSpecType, &NodeCordonedSpec{})
	if err != nil {
		panic(err)
	}
}
