// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// NodeAddressSortAlgorithmType is type of NodeAddressSortAlgorithm resource.
const NodeAddressSortAlgorithmType = resource.Type("NodeAddressSortAlgorithms.net.talos.dev")

// NodeAddressSortAlgorithm resource holds filter for NodeAddress resources.
type NodeAddressSortAlgorithm = typed.Resource[NodeAddressSortAlgorithmSpec, NodeAddressSortAlgorithmExtension]

// NodeAddressSortAlgorithmID is the resource ID for NodeAddressSortAlgorithm.
const NodeAddressSortAlgorithmID = "default"

// NodeAddressSortAlgorithmSpec describes a filter for NodeAddresses.
//
//gotagsrewrite:gen
type NodeAddressSortAlgorithmSpec struct {
	Algorithm nethelpers.AddressSortAlgorithm `yaml:"algorithm" protobuf:"1"`
}

// NewNodeAddressSortAlgorithm initializes a NodeAddressSortAlgorithm resource.
func NewNodeAddressSortAlgorithm(namespace resource.Namespace, id resource.ID) *NodeAddressSortAlgorithm {
	return typed.NewResource[NodeAddressSortAlgorithmSpec, NodeAddressSortAlgorithmExtension](
		resource.NewMetadata(namespace, NodeAddressSortAlgorithmType, id, resource.VersionUndefined),
		NodeAddressSortAlgorithmSpec{},
	)
}

// NodeAddressSortAlgorithmExtension provides auxiliary methods for NodeAddressSortAlgorithm.
type NodeAddressSortAlgorithmExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (NodeAddressSortAlgorithmExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeAddressSortAlgorithmType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Algorithm",
				JSONPath: `{.algorithm}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[NodeAddressSortAlgorithmSpec](NodeAddressSortAlgorithmType, &NodeAddressSortAlgorithm{})
	if err != nil {
		panic(err)
	}
}
