// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// IdentityType is type of Identity resource.
const IdentityType = resource.Type("Identities.cluster.talos.dev")

// LocalIdentity is the resource ID for the local node identity.
const LocalIdentity = resource.ID("local")

// Identity resource holds node identity (as a member of the cluster).
type Identity = typed.Resource[IdentitySpec, IdentityRD]

// IdentitySpec describes status of rendered secrets.
//
// Note: IdentitySpec is persisted on disk in the STATE partition,
// so YAML serialization should be kept backwards compatible.
//
//gotagsrewrite:gen
type IdentitySpec struct {
	// NodeID is a random value which is persisted across reboots,
	// but it gets reset on wipe.
	NodeID string `yaml:"nodeId" protobuf:"1"`
}

// NewIdentity initializes a Identity resource.
func NewIdentity(namespace resource.Namespace, id resource.ID) *Identity {
	return typed.NewResource[IdentitySpec, IdentityRD](
		resource.NewMetadata(namespace, IdentityType, id, resource.VersionUndefined),
		IdentitySpec{},
	)
}

// IdentityRD provides auxiliary methods for Identity.
type IdentityRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (c IdentityRD) ResourceDefinition(resource.Metadata, IdentitySpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             IdentityType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "ID",
				JSONPath: `{.nodeId}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[IdentitySpec](IdentityType, &Identity{})
	if err != nil {
		panic(err)
	}
}
