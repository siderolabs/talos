// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// DiscoveryRefreshStatusType is type of DiscoveryRefreshStatus resource.
const DiscoveryRefreshStatusType = resource.Type("DiscoveryRefreshStatuses.block.talos.dev")

// DiscoveryRefreshStatus resource holds a status of refresh.
type DiscoveryRefreshStatus = typed.Resource[DiscoveryRefreshStatusSpec, DiscoveryRefreshStatusExtension]

// DiscoveryRefreshStatusSpec is the spec for DiscoveryRefreshStatus status.
//
//gotagsrewrite:gen
type DiscoveryRefreshStatusSpec struct {
	Request int `yaml:"request" protobuf:"1"`
}

// NewDiscoveryRefreshStatus initializes a DiscoveryRefreshStatus resource.
func NewDiscoveryRefreshStatus(namespace resource.Namespace, id resource.ID) *DiscoveryRefreshStatus {
	return typed.NewResource[DiscoveryRefreshStatusSpec, DiscoveryRefreshStatusExtension](
		resource.NewMetadata(namespace, DiscoveryRefreshStatusType, id, resource.VersionUndefined),
		DiscoveryRefreshStatusSpec{},
	)
}

// DiscoveryRefreshStatusExtension is auxiliary resource data for DiscoveryRefreshStatus.
type DiscoveryRefreshStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (DiscoveryRefreshStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DiscoveryRefreshStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Request",
				JSONPath: `{.request}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[DiscoveryRefreshStatusSpec](DiscoveryRefreshStatusType, &DiscoveryRefreshStatus{})
	if err != nil {
		panic(err)
	}
}
