// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// IdentityType is type of Identity resource.
const IdentityType = resource.Type("Identities.cluster.talos.dev")

// LocalIdentity is the resource ID for the local node identity.
const LocalIdentity = resource.ID("local")

// Identity resource holds node identity (as a member of the cluster).
type Identity struct{}

// IdentitySpec describes status of rendered secrets.
//
// Note: IdentitySpec is persisted on disk in the STATE partition,
// so YAML serialization should be kept backwards compatible.
type IdentitySpec struct {
	// NodeID is a random value which is persisted across reboots,
	// but it gets reset on wipe.
	NodeID string `yaml:"nodeId"`
}

// NewIdentity initializes a Identity resource.
func NewIdentity(namespace resource.Namespace, id resource.ID) *TypedResource[IdentitySpec, Identity] {
	return NewTypedResource[IdentitySpec, Identity](
		resource.NewMetadata(namespace, IdentityType, id, resource.VersionUndefined),
		IdentitySpec{},
	)
}

func (Identity) String(md resource.Metadata, _ IdentitySpec) string {
	return fmt.Sprintf("cluster.Identity(%q)", md.ID())
}

// ResourceDefinition returns proper meta.ResourceDefinitionProvider for current type.
func (Identity) ResourceDefinition(_ resource.Metadata, _ IdentitySpec) meta.ResourceDefinitionSpec {
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
