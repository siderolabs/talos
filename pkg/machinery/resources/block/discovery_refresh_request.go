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

// DiscoveryRefreshRequestType is type of DiscoveryRefreshRequest resource.
const DiscoveryRefreshRequestType = resource.Type("DiscoveryRefreshRequests.block.talos.dev")

// RefreshID is the ID of the singleton discovery refresh request/statue.
const RefreshID resource.ID = "refresh"

// DiscoveryRefreshRequest resource holds a request to refresh the discovered volumes.
type DiscoveryRefreshRequest = typed.Resource[DiscoveryRefreshRequestSpec, DiscoveryRefreshRequestExtension]

// DiscoveryRefreshRequestSpec is the spec for DiscoveryRefreshRequest.
//
//gotagsrewrite:gen
type DiscoveryRefreshRequestSpec struct {
	Request int `yaml:"request" protobuf:"1"`
}

// NewDiscoveryRefreshRequest initializes a DiscoveryRefreshRequest resource.
func NewDiscoveryRefreshRequest(namespace resource.Namespace, id resource.ID) *DiscoveryRefreshRequest {
	return typed.NewResource[DiscoveryRefreshRequestSpec, DiscoveryRefreshRequestExtension](
		resource.NewMetadata(namespace, DiscoveryRefreshRequestType, id, resource.VersionUndefined),
		DiscoveryRefreshRequestSpec{},
	)
}

// DiscoveryRefreshRequestExtension is auxiliary resource data for DiscoveryRefreshRequest.
type DiscoveryRefreshRequestExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (DiscoveryRefreshRequestExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DiscoveryRefreshRequestType,
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

	err := protobuf.RegisterDynamic[DiscoveryRefreshRequestSpec](DiscoveryRefreshRequestType, &DiscoveryRefreshRequest{})
	if err != nil {
		panic(err)
	}
}
