// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// NodeStatusType is type of NodeStatus resource.
const NodeStatusType = resource.Type("NodeStatuses.kubernetes.talos.dev")

// NodeStatus resource holds Kubernetes NodeStatus.
type NodeStatus = typed.Resource[NodeStatusSpec, NodeStatusExtension]

// NodeStatusSpec describes Kubernetes NodeStatus.
//
//gotagsrewrite:gen
type NodeStatusSpec struct {
	Nodename      string            `yaml:"nodename" protobuf:"1"`
	NodeReady     bool              `yaml:"nodeReady" protobuf:"2"`
	Unschedulable bool              `yaml:"unschedulable" protobuf:"3"`
	Labels        map[string]string `yaml:"labels" protobuf:"4"`
	Annotations   map[string]string `yaml:"annotations" protobuf:"5"`
	PodCIDRs      []netip.Prefix    `yaml:"podCIDRs" protobuf:"6"`
}

// NewNodeStatus initializes a NodeStatus resource.
func NewNodeStatus(namespace resource.Namespace, id resource.ID) *NodeStatus {
	return typed.NewResource[NodeStatusSpec, NodeStatusExtension](
		resource.NewMetadata(namespace, NodeStatusType, id, resource.VersionUndefined),
		NodeStatusSpec{},
	)
}

// NodeStatusExtension provides auxiliary methods for NodeStatus.
type NodeStatusExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (NodeStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Ready",
				JSONPath: "{.nodeReady}",
			},
			{
				Name:     "Unschedulable",
				JSONPath: "{.unschedulable}",
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[NodeStatusSpec](NodeStatusType, &NodeStatus{})
	if err != nil {
		panic(err)
	}
}
