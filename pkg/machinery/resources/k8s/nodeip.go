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

// NodeIPType is type of NodeIP resource.
const NodeIPType = resource.Type("NodeIPs.kubernetes.talos.dev")

// NodeIP resource holds definition of Node IP specification.
type NodeIP = typed.Resource[NodeIPSpec, NodeIPRD]

// NodeIPSpec holds the Node IP specification.
//
//gotagsrewrite:gen
type NodeIPSpec struct {
	Addresses []netip.Addr `yaml:"addresses" protobuf:"1"`
}

// NewNodeIP initializes an empty NodeIP resource.
func NewNodeIP(namespace resource.Namespace, id resource.ID) *NodeIP {
	return typed.NewResource[NodeIPSpec, NodeIPRD](
		resource.NewMetadata(namespace, NodeIPType, id, resource.VersionUndefined),
		NodeIPSpec{},
	)
}

// NodeIPRD provides auxiliary methods for NodeIP.
type NodeIPRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (NodeIPRD) ResourceDefinition(resource.Metadata, NodeIPSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodeIPType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[NodeIPSpec](NodeIPType, &NodeIP{})
	if err != nil {
		panic(err)
	}
}
